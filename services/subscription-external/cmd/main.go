// @title TIMWE Subscription External Service API
// @version 1.0
// @description |
//   # TIMWE Subscription External Service API
//
//   This service provides comprehensive subscription management capabilities including:
//
//   ## Core Features
//   - **Subscription Management**: Opt-in, opt-out, and subscription status management
//   - **Batch Processing**: High-volume subscription operations with progress tracking
//   - **Charging Failure Analysis**: Advanced monitoring and analysis of charging failures
//   - **Real-time Monitoring**: Comprehensive monitoring dashboard with alerts
//   - **Batch Processing Worker**: Automated resubscription processing with configurable workflows
//   - **Renewal System**: Automated opt-out/opt-in renewal processing with churn management
//
//   ## Key Components
//
//   ### Charging Failure Management
//   - Identify and analyze charging failures from notifications data
//   - Track charging health status and failure reasons
//   - Generate comprehensive statistics and summaries
//   - Mark failures as processed with audit trail
//
//   ### Monitoring & Alerting
//   - Real-time metrics collection and visualization
//   - Configurable alert thresholds and notifications
//   - Health status monitoring with detailed checks
//   - Dashboard with charts and real-time data
//
//   ### Worker System
//   - Automated batch processing of failed subscriptions
//   - Configurable concurrency and retry logic
//   - Progress tracking and result management
//   - Priority-based processing with health updates
//
//   ### Renewal System
//   - Automated opt-out/opt-in renewal processing
//   - Intelligent churn policy evaluation and management
//   - Priority retry queue for failed operations
//   - Comprehensive monitoring and alerting
//   - Configurable renewal strategies and policies
//
//   ## Authentication
//   All endpoints require proper authentication and authorization.
//
//   ## Rate Limiting
//   Endpoints are subject to rate limiting to ensure service stability.
//
//   ## Support
//   For technical support, contact the development team.
// @termsOfService https://omni-connect.com/terms/
// @contact.name API Support
// @contact.url https://omni-connect.com/support
// @contact.email support@omni-connect.com
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:8083
// @BasePath /
// @schemes http https

//go:generate swag init -g main.go -d .,../internal,../internal/handler,../internal/service,../internal/transport,../internal/worker,../internal/domain -o ../docs --instanceName swagger
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"database/sql"

	cached "github.com/seidu626/subscription-manager/common/cache"
	"github.com/seidu626/subscription-manager/common/config"
	_ "github.com/seidu626/subscription-manager/subscription-external/docs"
	renewalconfig "github.com/seidu626/subscription-manager/subscription-external/internal/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/handler"
	"github.com/seidu626/subscription-manager/subscription-external/internal/logging"
	"github.com/seidu626/subscription-manager/subscription-external/internal/middleware"
	"github.com/seidu626/subscription-manager/subscription-external/internal/monitoring"
	"github.com/seidu626/subscription-manager/subscription-external/internal/repository"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
	"github.com/seidu626/subscription-manager/subscription-external/internal/transport"
	"github.com/seidu626/subscription-manager/subscription-external/internal/utils"
	"github.com/seidu626/subscription-manager/subscription-external/internal/worker"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// syncWorkerStats syncs worker processing statistics with the monitor
func syncWorkerStats(monitor *monitoring.ChargingFailureMonitor, processor *worker.ResubscriptionProcessor, logger *zap.Logger) {
	if processor == nil {
		logger.Warn("Processor is nil, skipping worker stats sync")
		return
	}

	logger.Info("Starting worker stats sync...")

	stats := processor.GetStats()
	logger.Info("Retrieved worker stats", zap.Any("stats", stats))

	// Update only processing-related metrics
	processingQueue := int64(stats.CurrentBatch)
	processedToday := stats.TotalProcessed
	successRate := stats.SuccessRate
	averageProcessingTime := stats.AverageTime.Seconds()

	// Get processing status
	processingStatus := processor.GetStatus()

	logger.Info("Extracted worker metrics",
		zap.Int64("processing_queue", processingQueue),
		zap.Int64("processed_today", processedToday),
		zap.Float64("success_rate", successRate),
		zap.Float64("average_processing_time", averageProcessingTime),
		zap.String("processing_status", processingStatus))

	monitor.UpdateProcessingMetrics(processingQueue, processedToday, successRate, averageProcessingTime, processingStatus)

	logger.Info("Worker stats synced successfully",
		zap.Int64("total_processed", stats.TotalProcessed),
		zap.Float64("success_rate", stats.SuccessRate),
		zap.String("status", processingStatus))
}

// syncMonitorData syncs real data from the database with the monitor
func syncMonitorData(monitor *monitoring.ChargingFailureMonitor, repo repository.SubscriptionRepositoryInterface, logger *zap.Logger) error {
	logger.Info("Starting monitor data sync...")

	// Create circuit breakers for database operations
	dbCircuitBreaker := monitoring.NewCircuitBreaker(monitoring.CircuitBreakerConfig{
		Name:         "database_operations",
		MaxFailures:  3,
		Timeout:      30 * time.Second,
		ResetTimeout: 60 * time.Second,
	}, logger)

	// Test database connectivity first
	logger.Info("Testing database connectivity...")

	var totalSubscriptions int64
	var stats, summary map[string]interface{}

	// Get total subscriptions count with circuit breaker
	err := dbCircuitBreaker.Execute(func() error {
		var err error
		totalSubscriptions, err = repo.GetTotalSubscriptionsCount()
		return err
	})
	if err != nil {
		logger.Warn("Failed to get total subscriptions count, using 0", zap.Error(err))
		totalSubscriptions = 0
	} else {
		logger.Info("Retrieved total subscriptions count", zap.Int64("count", totalSubscriptions))
	}

	// Get charging failure statistics with circuit breaker
	err = dbCircuitBreaker.Execute(func() error {
		var err error
		stats, err = repo.GetChargingFailureStats()
		return err
	})
	if err != nil {
		logger.Error("Failed to get charging failure stats", zap.Error(err))
		// Use empty stats as fallback
		stats = make(map[string]interface{})
	} else {
		logger.Info("Retrieved charging failure stats", zap.Any("stats", stats))
		// Debug: Print the exact type and value of total_charging_failures
		if val, exists := stats["total_charging_failures"]; exists {
			logger.Info("Debug total_charging_failures",
				zap.Any("value", val),
				zap.String("type", fmt.Sprintf("%T", val)))
		} else {
			logger.Warn("total_charging_failures key not found in stats")
		}
	}

	// Get charging failure summary with circuit breaker
	err = dbCircuitBreaker.Execute(func() error {
		var err error
		summary, err = repo.GetChargingFailureSummary()
		return err
	})
	if err != nil {
		logger.Error("Failed to get charging failure summary", zap.Error(err))
		// Use empty summary as fallback
		summary = make(map[string]interface{})
	} else {
		logger.Info("Retrieved charging failure summary", zap.Any("summary", summary))
		// Debug: Print the exact structure of NEVER_CHARGED
		if val, exists := summary["NEVER_CHARGED"]; exists {
			logger.Info("Debug NEVER_CHARGED",
				zap.Any("value", val),
				zap.String("type", fmt.Sprintf("%T", val)))
			if mapVal, ok := val.(map[string]interface{}); ok {
				if count, exists := mapVal["count"]; exists {
					logger.Info("Debug NEVER_CHARGED count",
						zap.Any("count_value", count),
						zap.String("count_type", fmt.Sprintf("%T", count)))
				}
			}
		} else {
			logger.Warn("NEVER_CHARGED key not found in summary")
		}
	}

	// Extract metrics from the stats
	var chargingFailures, neverCharged, staleCharges, chargingRecent, chargingDelayed int64
	var failureRate, successRate float64

	// Helper function to extract int64 from various numeric types
	extractInt64 := func(value interface{}) (int64, bool) {
		switch v := value.(type) {
		case int64:
			return v, true
		case int:
			return int64(v), true
		case float64:
			return int64(v), true
		case float32:
			return int64(v), true
		default:
			return 0, false
		}
	}

	if totalCount, ok := extractInt64(stats["total_charging_failures"]); ok {
		chargingFailures = totalCount
		logger.Info("Extracted total charging failures", zap.Int64("count", chargingFailures))
	} else {
		logger.Warn("Could not extract total_charging_failures from stats", zap.Any("stats", stats))
	}

	if summaryData, ok := summary["NEVER_CHARGED"].(map[string]interface{}); ok {
		if count, ok := extractInt64(summaryData["count"]); ok {
			neverCharged = count
			logger.Info("Extracted NEVER_CHARGED count", zap.Int64("count", neverCharged))
		}
	} else {
		logger.Warn("Could not extract NEVER_CHARGED from summary", zap.Any("summary", summary))
	}

	if summaryData, ok := summary["CHARGING_STALE"].(map[string]interface{}); ok {
		if count, ok := extractInt64(summaryData["count"]); ok {
			staleCharges = count
			logger.Info("Extracted CHARGING_STALE count", zap.Int64("count", staleCharges))
		}
	} else {
		logger.Warn("Could not extract CHARGING_STALE from summary", zap.Any("summary", summary))
	}

	if summaryData, ok := summary["CHARGING_RECENT"].(map[string]interface{}); ok {
		if count, ok := extractInt64(summaryData["count"]); ok {
			chargingRecent = count
			logger.Info("Extracted CHARGING_RECENT count", zap.Int64("count", chargingRecent))
		}
	} else {
		logger.Warn("Could not extract CHARGING_RECENT from summary", zap.Any("summary", summary))
	}

	if summaryData, ok := summary["CHARGING_DELAYED"].(map[string]interface{}); ok {
		if count, ok := extractInt64(summaryData["count"]); ok {
			chargingDelayed = count
			logger.Info("Extracted CHARGING_DELAYED count", zap.Int64("count", chargingDelayed))
		}
	} else {
		logger.Warn("Could not extract CHARGING_DELAYED from summary", zap.Any("summary", summary))
	}

	// Calculate failure rate
	if totalSubscriptions > 0 {
		failureRate = float64(chargingFailures) / float64(totalSubscriptions) * 100
		successRate = 100 - failureRate
		logger.Info("Calculated rates", zap.Float64("failure_rate", failureRate), zap.Float64("success_rate", successRate))
	} else {
		logger.Warn("Total subscriptions is 0, cannot calculate rates")
	}

	// Create metrics object
	metrics := &monitoring.ChargingFailureMetrics{
		TotalSubscriptions:    totalSubscriptions,
		ChargingFailures:      chargingFailures,
		FailureRate:           failureRate,
		NeverCharged:          neverCharged,
		StaleCharges:          staleCharges,
		ChargingRecent:        chargingRecent,
		ChargingDelayed:       chargingDelayed,
		ChargingStale:         staleCharges, // Add the missing ChargingStale field
		ProcessingQueue:       0,            // Will be updated by worker
		ProcessedToday:        0,            // Will be updated by worker
		SuccessRate:           successRate,
		LastUpdated:           time.Now(),
		ProcessingStatus:      "idle",                       // Will be updated by worker
		AverageProcessingTime: 0,                            // Will be updated by worker
		Metadata:              make(map[string]interface{}), // Add the missing Metadata field
	}

	// Update the monitor with real data
	monitor.UpdateMetrics(metrics)

	// Verify the update worked by getting the metrics back
	updatedMetrics := monitor.GetMetrics()
	logger.Info("Verified monitor metrics after update",
		zap.Int64("total_subscriptions", updatedMetrics.TotalSubscriptions),
		zap.Int64("charging_failures", updatedMetrics.ChargingFailures),
		zap.Float64("failure_rate", updatedMetrics.FailureRate),
		zap.Int64("never_charged", updatedMetrics.NeverCharged))

	// Additional verification - check if the data persists
	time.Sleep(1 * time.Second) // Wait a bit
	persistentMetrics := monitor.GetMetrics()
	logger.Info("Verified monitor metrics persistence after 1 second",
		zap.Int64("total_subscriptions", persistentMetrics.TotalSubscriptions),
		zap.Int64("charging_failures", persistentMetrics.ChargingFailures),
		zap.Float64("failure_rate", persistentMetrics.FailureRate),
		zap.Int64("never_charged", persistentMetrics.NeverCharged))

	logger.Info("Monitor data synced successfully",
		zap.Int64("total_subscriptions", totalSubscriptions),
		zap.Int64("charging_failures", chargingFailures),
		zap.Int64("never_charged", neverCharged),
		zap.Int64("charging_recent", chargingRecent),
		zap.Int64("charging_delayed", chargingDelayed),
		zap.Int64("charging_stale", staleCharges),
		zap.Float64("failure_rate", failureRate))

	return nil
}

func validateTIMWEStartupConfig(cfg *config.Config) error {
	apiKey := strings.TrimSpace(cfg.Application.TIMWE.APIKey)
	psk := strings.TrimSpace(cfg.Application.TIMWE.Psk)
	partnerServiceID := strings.TrimSpace(cfg.Application.TIMWE.PartnerServiceID)
	authKey := strings.TrimSpace(cfg.Application.TIMWE.AuthenticationKey)

	if apiKey == "" {
		return errors.New("TIMWE API key is missing")
	}

	// If static authentication key is configured, allow bypassing PSK-based auth key generation.
	if authKey != "" {
		return nil
	}

	if partnerServiceID == "" {
		return errors.New("TIMWE partner service ID is missing")
	}

	if !isValidAESKeyLength(psk) {
		return fmt.Errorf("TIMWE PSK length is invalid: %d", len(psk))
	}

	return nil
}

func isValidAESKeyLength(key string) bool {
	switch len(key) {
	case 16, 24, 32:
		return true
	default:
		return false
	}
}

func main() {
	// Initialize basic logger first for config loading
	basicLogger, err := logging.NewZapLogger("")
	if err != nil {
		log.Fatalf("could not initialize basic logger: %v", err)
	}

	// Load configuration
	cfg := config.InitConfig(basicLogger, ".", []string{"config.yaml"})

	// Initialize rolling file logger if enabled
	var logger *zap.Logger
	if cfg.Application.Log.Rolling.Enabled {
		rollingConfig := logging.RollingLogConfig{
			Enabled:           cfg.Application.Log.Rolling.Enabled,
			MaxSize:           cfg.Application.Log.Rolling.MaxSize,
			MaxAge:            cfg.Application.Log.Rolling.MaxAge,
			MaxBackups:        cfg.Application.Log.Rolling.MaxBackups,
			Compress:          cfg.Application.Log.Rolling.Compress,
			CompressThreshold: cfg.Application.Log.Rolling.CompressThreshold,
			LocalTime:         cfg.Application.Log.Rolling.LocalTime,
		}

		logger, err = logging.NewRollingFileLogger(cfg.Application.Log.Path, rollingConfig)
		if err != nil {
			log.Fatalf("could not initialize rolling file logger: %v", err)
		}
	} else {
		// Fallback to basic logger
		logger = basicLogger
	}

	defer func() {
		_ = logger.Sync() // flushes buffer, if any
	}()

	if err := validateTIMWEStartupConfig(cfg); err != nil {
		logger.Fatal("Invalid TIMWE startup configuration",
			zap.Error(err),
			zap.Bool("has_api_key", strings.TrimSpace(cfg.Application.TIMWE.APIKey) != ""),
			zap.Int("psk_length", len(strings.TrimSpace(cfg.Application.TIMWE.Psk))),
			zap.Bool("has_partner_service_id", strings.TrimSpace(cfg.Application.TIMWE.PartnerServiceID) != ""),
			zap.Bool("has_authentication_key", strings.TrimSpace(cfg.Application.TIMWE.AuthenticationKey) != ""),
		)
	}

	if cfg.Application.Environment == config.PRODUCTION {
		logger, _ = zap.NewProduction()
	}
	defer func(logger *zap.Logger) {
		err := logger.Sync()
		if err != nil {
		}
	}(logger)

	// Initialize panic handler
	if err := utils.InitPanicHandler(logger); err != nil {
		logger.Error("Failed to initialize panic handler", zap.Error(err))
		// Continue with default panic handling
	}

	// Add panic recovery to main function
	defer func() {
		if r := recover(); r != nil {
			logger.Error("MAIN FUNCTION PANIC",
				zap.Any("panic_value", r),
				zap.String("panic_type", fmt.Sprintf("%T", r)),
				zap.String("timestamp", time.Now().Format(time.RFC3339)),
			)

			// Use panic handler if available
			if panicHandler := utils.GetGlobalPanicHandler(); panicHandler != nil {
				panicHandler.HandlePanic(r, context.Background())
			} else {
				// Fallback: exit with error
				logger.Fatal("Application terminating due to panic in main function",
					zap.Any("panic_value", r),
				)
				os.Exit(1)
			}
		}
	}()

	// Get the connection string from the config
	connStr := config.GetDBConnectionString()
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Fatalf("Failed to close the connection to the database: %v", err)
		}
	}(db)

	redisOptions := config.GetRedisOptions()
	redisClient := cached.NewFailoverRedisClient(redisOptions)
	logger.Info("cache client initialized", zap.String("mode", string(redisClient.Mode())))

	repo := repository.NewSubscriptionRepository(db, logger, redisClient)
	productRepo := repository.NewProductRepository(db, logger, redisClient)
	userBaseRepo := repository.NewUserBaseRepository(db, logger, redisClient)
	// Initialize renewal system first
	_ = repository.NewRenewalRepository(repo.GetDB(), logger) // Not directly used but kept for reference

	// Load renewal configuration
	renewalConfig, err := renewalconfig.LoadRenewalConfig("config/renewal.yaml")
	if err != nil {
		logger.Warn("Failed to load renewal config, using defaults", zap.Error(err))
		renewalConfig, _ = renewalconfig.LoadRenewalConfig("")
	}

	// Create renewal service with nil subscription service initially (will be set later if needed)
	renewalService := service.NewRenewalService(nil, repo, productRepo, logger, renewalConfig)

	// Create subscription service with renewal service dependency
	svc := service.NewSubscriptionService(logger, repo, productRepo, userBaseRepo, cfg, renewalService)
	userBaseSvc := service.NewUserBaseService(logger, userBaseRepo, cfg)
	subscriptionHandler := handler.NewSubscriptionHandler(logger, svc, cfg)
	userBaseHandler := handler.NewUserBaseHandler(logger, userBaseSvc, cfg)
	partnerHandler := handler.NewPartnerHandler(logger, svc, cfg).WithTenantRepo(repo)

	// Initialize monitoring and worker components
	monitor := monitoring.NewChargingFailureMonitor(logger)

	// Create enhanced monitoring system
	enhancedMonitor := monitoring.NewEnhancedMonitor(logger, monitor)

	// Create configuration manager
	configManager := monitoring.NewConfigManager("config/monitoring.yaml", logger)

	// Create system health monitor
	healthConfig := &monitoring.HealthConfig{
		CheckInterval:   30 * time.Second,
		Timeout:         10 * time.Second,
		MaxFailures:     3,
		EnableMetrics:   true,
		EnableAlerts:    true,
		HealthCheckPath: "/health",
	}
	healthMonitor := monitoring.NewSystemHealthMonitor(healthConfig, logger)

	// Create metrics cache
	cacheConfig := &monitoring.CacheConfig{
		DefaultTTL:      5 * time.Minute,
		MaxEntries:      1000,
		CleanupInterval: 1 * time.Minute,
		EnableStats:     true,
		Compression:     false,
	}
	metricsCache := monitoring.NewMetricsCache(cacheConfig, logger)

	// Create monitoring handler
	monitoringHandler := handler.NewMonitoringHandler(monitor, logger, cfg)

	// Start the monitor with context
	monitorCtx := context.Background()

	// Start configuration manager
	if err := configManager.Start(monitorCtx); err != nil {
		logger.Fatal("Failed to start configuration manager", zap.Error(err))
	}
	logger.Info("Configuration manager started successfully")

	// Start system health monitor
	if err := healthMonitor.Start(monitorCtx); err != nil {
		logger.Fatal("Failed to start system health monitor", zap.Error(err))
	}
	logger.Info("System health monitor started successfully")

	// Register health checkers
	dbHealthChecker := monitoring.NewDatabaseHealthChecker(repo.GetDB(), "postgres", logger)
	healthMonitor.RegisterHealthChecker(dbHealthChecker)

	// Register service health checkers
	serviceHealthChecker := monitoring.NewServiceHealthChecker("monitoring_service", func() error {
		// Simple health check - verify monitor is running
		if !monitor.IsRunning() {
			return fmt.Errorf("monitor is not running")
		}
		return nil
	}, logger)
	healthMonitor.RegisterHealthChecker(serviceHealthChecker)

	// Register real-time service health checker
	realtimeHealthChecker := monitoring.NewServiceHealthChecker("realtime_service", func() error {
		if !monitor.IsRunning() {
			return fmt.Errorf("real-time monitor is not running")
		}
		return nil
	}, logger)
	healthMonitor.RegisterHealthChecker(realtimeHealthChecker)

	// Start metrics cache
	if err := metricsCache.Start(monitorCtx); err != nil {
		logger.Fatal("Failed to start metrics cache", zap.Error(err))
	}
	logger.Info("Metrics cache started successfully")

	// Start the real-time monitor
	realTimeMonitor := monitoringHandler.GetRealTimeMonitor()
	if err := realTimeMonitor.Start(monitorCtx); err != nil {
		logger.Fatal("Failed to start real-time monitor", zap.Error(err))
	}
	logger.Info("Real-time monitor started successfully")

	// Connect the real-time monitor to the charging failure monitor
	monitor.SetRealTimeMonitor(realTimeMonitor)

	// Connect the health monitor to the charging failure monitor
	monitor.SetHealthMonitor(healthMonitor)

	// Start the charging failure monitor
	if err := monitor.Start(monitorCtx); err != nil {
		logger.Fatal("Failed to start charging failure monitor", zap.Error(err))
	}
	logger.Info("Charging failure monitor started successfully")

	// Start the enhanced monitoring system
	if err := enhancedMonitor.Start(monitorCtx); err != nil {
		logger.Fatal("Failed to start enhanced monitor", zap.Error(err))
	}
	logger.Info("Enhanced monitoring system started successfully")

	processor := worker.NewResubscriptionProcessor(repo, svc, monitor, logger, nil)
	workerHandler := handler.NewWorkerHandler(processor, logger)

	// Create renewal worker and handler
	renewalWorker := worker.NewRenewalWorker(renewalService, repo, productRepo, logger, renewalConfig)
	renewalHandler := handler.NewRenewalHandler(renewalService, renewalWorker, logger)

	// Start a goroutine to sync real data with the monitor
	go func() {
		// Do initial sync immediately
		logger.Info("Performing initial data sync...")
		if err := syncMonitorData(monitor, repo, logger); err != nil {
			logger.Error("Failed to perform initial data sync", zap.Error(err))
		}
		syncWorkerStats(monitor, processor, logger)

		ticker := time.NewTicker(30 * time.Second) // Sync every 30 seconds
		defer ticker.Stop()

		for {
			select {
			case <-monitorCtx.Done():
				return
			case <-ticker.C:
				if err := syncMonitorData(monitor, repo, logger); err != nil {
					logger.Error("Failed to sync monitor data", zap.Error(err))
				}
				// Also sync worker stats
				syncWorkerStats(monitor, processor, logger)
			}
		}
	}()

	router := transport.NewRouter(subscriptionHandler, userBaseHandler, partnerHandler, monitoringHandler, workerHandler, renewalHandler)

	// Wrap router with panic recovery middleware
	panicMiddleware := middleware.NewPanicRecoveryMiddleware(logger, utils.GetGlobalPanicHandler())
	wrappedRouter := panicMiddleware.WrapFastHTTPWithMetrics(router, "main-router")

	handlerWithCORS := middleware.CORSMiddleware(wrappedRouter, cfg.Application.AllowedOrigins)

	server := &fasthttp.Server{
		Handler:            handlerWithCORS,
		ReadTimeout:        cfg.Application.HTTP.ReadTimeout,
		WriteTimeout:       cfg.Application.HTTP.WriteTimeout,
		IdleTimeout:        cfg.Application.HTTP.IdleTimeout,
		MaxRequestBodySize: cfg.Application.HTTP.MaxRequestBodyMB * 1024 * 1024,
		Concurrency:        cfg.Application.HTTP.Concurrency,
		ReduceMemoryUsage:  true,
	}

	// Sensible defaults if zero values were provided
	if server.ReadTimeout == 0 {
		server.ReadTimeout = 60 * 1e9 // 60s
	}
	if server.WriteTimeout == 0 {
		server.WriteTimeout = 60 * 1e9
	}
	if server.IdleTimeout == 0 {
		server.IdleTimeout = 120 * 1e9
	}
	if server.MaxRequestBodySize == 0 {
		server.MaxRequestBodySize = 16 * 1024 * 1024
	}

	log.Printf("Starting subscription management service on port: %d...", cfg.Application.Port)

	// Set up signal handling for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(fmt.Sprintf(":%d", cfg.Application.Port)); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	<-quit
	log.Println("Shutting down server...")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.ShutdownWithContext(shutdownCtx); err != nil {
		log.Printf("Failed to shutdown server: %v", err)
	}
	log.Println("Server stopped gracefully")
}
