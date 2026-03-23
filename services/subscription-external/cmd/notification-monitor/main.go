package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	cached "github.com/seidu626/subscription-manager/common/cache"
	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/repository"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
	"github.com/seidu626/subscription-manager/subscription-external/internal/worker"
	"go.uber.org/zap"

	_ "github.com/lib/pq"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
)

func main() {
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	cfg := config.InitConfig(logger, ".", []string{"config.yaml"})

	connStr := config.GetDBConnectionString()
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	redisOptions := config.GetRedisOptions()
	redisClient := cached.NewFailoverRedisClient(redisOptions)
	logger.Info("cache client initialized", zap.String("mode", string(redisClient.Mode())))
	if redisClient.Mode() == cached.RedisModeFallback && os.Getenv("APP_SINGLE_INSTANCE") != "true" {
		logger.Warn("running notification monitor with in-memory cache fallback; distributed locks are local-only. Set APP_SINGLE_INSTANCE=true to avoid multi-instance duplicate processing")
	}

	repo := repository.NewSubscriptionRepository(db, logger, redisClient)
	productRepo := repository.NewProductRepository(db, logger, redisClient)
	userBaseRepo := repository.NewUserBaseRepository(db, logger, redisClient)

	// Initialize renewal system
	_ = repository.NewRenewalRepository(repo.GetDB(), logger) // Not directly used but may be needed for future expansion

	// Load renewal configuration (use defaults if file not found)
	renewalConfig := &domain.RenewalConfig{
		Strategy: domain.StrategyOptOutOptIn,
		Enabled:  true,
		ChurnPolicy: domain.ChurnPolicy{
			MaxHoursWithoutPayment: 168, // 7 days * 24 hours
			MaxRenewalAttempts:     3,
			RetryIntervalHours:     24,
			GracePeriodHours:       48, // 2 days * 24 hours
		},
	}

	// Set embedded struct fields
	renewalConfig.OptOutOptIn.WaitBetweenMs = 3000
	renewalConfig.OptOutOptIn.BatchSize = 50
	renewalConfig.OptOutOptIn.MaxConcurrent = 5
	renewalConfig.OptOutOptIn.RateLimitMs = 500
	renewalConfig.OptOutOptIn.BatchDelayMs = 2000

	renewalConfig.Worker.Enabled = true
	renewalConfig.Worker.DailyRunTime = "02:00"
	renewalConfig.Worker.Timezone = "UTC"
	renewalConfig.Worker.MaxRetries = 3
	renewalConfig.Worker.TimeoutPerRenewal = 30 * time.Second

	renewalConfig.Monitoring.AlertOnFailureRate = 0.3
	renewalConfig.Monitoring.AlertOnChurnRate = 0.1
	renewalConfig.Monitoring.MetricsPort = 9090

	// Create renewal service with nil subscription service initially (will be set later)
	renewalService := service.NewRenewalService(nil, repo, productRepo, logger, renewalConfig)

	svc := service.NewSubscriptionService(logger, repo, productRepo, userBaseRepo, cfg, renewalService)

	// metrics endpoint
	worker.RegisterMetrics()
	go func() {
		port := os.Getenv("METRICS_PORT")
		if port == "" {
			port = "9099"
		}
		http.Handle("/metrics", promhttp.Handler())
		log.Printf("metrics listening on :%s", port)
		_ = http.ListenAndServe(":"+port, nil)
	}()

	// Load notification monitor configuration
	monitorConfigPath := os.Getenv("NOTIFICATION_MONITOR_CONFIG")
	if monitorConfigPath == "" {
		monitorConfigPath = "config/notification-monitor.yaml"
	}

	monitorConfig, err := worker.LoadNotificationMonitorConfig(monitorConfigPath)
	if err != nil {
		logger.Warn("Failed to load notification monitor config, using defaults", zap.Error(err))
		// Use default configuration
		monitorConfig = &worker.NotificationMonitorConfig{
			BatchSize:           2000,
			MaxInFlightBatches:  20,
			ScanLookbackDays:    90,
			RenewalWindowMonths: 2,
			IdleSleep:           3 * time.Second,
			LeaseTTL:            30 * time.Second,
			RedisKeyPrefix:      fmt.Sprintf("notifmon:%s", cfg.Application.Environment),
			ProductIds:          []string{"8509", "14392", "14396", "14397", "14398", "27188", "14439"},
			EntryChannels:       []string{"USSD", "SMS", "WEB"},
			DefaultEntryChannel: "USSD",
		}
	}

	// Validate configuration
	if err := monitorConfig.ValidateConfig(); err != nil {
		logger.Fatal("Invalid notification monitor configuration", zap.Error(err))
	}

	// Override Redis key prefix with environment
	monitorConfig.RedisKeyPrefix = fmt.Sprintf("%s:%s", monitorConfig.RedisKeyPrefix, cfg.Application.Environment)

	logger.Info("Starting notification monitor with configuration",
		zap.Int("batchSize", monitorConfig.BatchSize),
		zap.Int("maxInFlightBatches", monitorConfig.MaxInFlightBatches),
		zap.Int("scanLookbackDays", monitorConfig.ScanLookbackDays),
		zap.Strings("productIds", monitorConfig.ProductIds),
		zap.Strings("entryChannels", monitorConfig.EntryChannels),
		zap.String("defaultEntryChannel", monitorConfig.DefaultEntryChannel))

	mon := worker.NewNotificationMonitor(logger, repo, svc, redisClient, *monitorConfig)

	// Wire acquisition client for charge-success postback pipeline
	acquisitionClient := service.NewAcquisitionClient(logger)
	mon.WithAcquisitionClient(acquisitionClient)

	if err := mon.Run(); err != nil {
		log.Fatalf("monitor exited with error: %v", err)
	}
}
