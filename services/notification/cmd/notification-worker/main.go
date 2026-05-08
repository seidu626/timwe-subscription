package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/seidu626/subscription-manager/notification/internal/config"
	"github.com/seidu626/subscription-manager/notification/internal/dispatcher"
	"github.com/seidu626/subscription-manager/notification/internal/repository"
	"go.uber.org/zap"
)

const defaultMetricsAddr = ":9103"

func main() {
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	cfg := config.InitConfig(logger, ".", []string{"config.yaml", ".env"})

	connStr := config.GetDBConnectionString(cfg)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	repo := repository.NewOutboxRepository(db, logger)
	workerCfg := loadWorkerConfig()
	dispatcher.RegisterMetrics()
	worker := dispatcher.NewDispatcher(repo, logger, workerCfg)
	metricsAddr := getEnvString("NOTIFICATION_WORKER_METRICS_ADDR", defaultMetricsAddr)
	metricsSrv := startMetricsServer(logger, metricsAddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer shutdownMetricsServer(logger, metricsSrv)

	go func() {
		if err := worker.Run(ctx); err != nil && err != context.Canceled {
			logger.Error("notification worker stopped", zap.Error(err))
		}
	}()

	logger.Info("notification worker started",
		zap.Int("batch_size", workerCfg.BatchSize),
		zap.Duration("poll_interval", workerCfg.PollInterval),
		zap.Int("max_attempts", workerCfg.MaxAttempts),
		zap.String("metrics_addr", metricsAddr),
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("notification worker shutting down")
	cancel()
	time.Sleep(2 * time.Second)
}

func startMetricsServer(logger *zap.Logger, addr string) *http.Server {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: promhttp.Handler(),
	}

	go func() {
		logger.Info("notification worker metrics endpoint started", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Warn("notification worker metrics endpoint stopped", zap.String("addr", addr), zap.Error(err))
		}
	}()

	return srv
}

func shutdownMetricsServer(logger *zap.Logger, srv *http.Server) {
	if srv == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Warn("notification worker metrics endpoint shutdown failed", zap.String("addr", srv.Addr), zap.Error(err))
	}
}

func loadWorkerConfig() dispatcher.Config {
	return dispatcher.Config{
		BatchSize:    getEnvInt("NOTIFICATION_WORKER_BATCH_SIZE", 200),
		PollInterval: getEnvDuration("NOTIFICATION_WORKER_POLL_INTERVAL", 5*time.Second),
		MaxAttempts:  getEnvInt("NOTIFICATION_WORKER_MAX_ATTEMPTS", 5),
		BackoffBase:  getEnvDuration("NOTIFICATION_WORKER_BACKOFF_BASE", 5*time.Second),
		BackoffMax:   getEnvDuration("NOTIFICATION_WORKER_BACKOFF_MAX", time.Hour),
		MTBaseURL:    os.Getenv("NOTIFICATION_WORKER_MT_BASE_URL"),
		MTChannel:    os.Getenv("NOTIFICATION_WORKER_MT_CHANNEL"),
		HTTPTimeout:  getEnvDuration("NOTIFICATION_WORKER_HTTP_TIMEOUT", 30*time.Second),
	}
}

func getEnvString(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return fallback
}
