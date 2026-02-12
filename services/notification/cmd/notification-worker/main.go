package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/seidu626/subscription-manager/notification/internal/config"
	"github.com/seidu626/subscription-manager/notification/internal/dispatcher"
	"github.com/seidu626/subscription-manager/notification/internal/repository"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

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
	worker := dispatcher.NewDispatcher(repo, logger, workerCfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := worker.Run(ctx); err != nil && err != context.Canceled {
			logger.Error("notification worker stopped", zap.Error(err))
		}
	}()

	logger.Info("notification worker started",
		zap.Int("batch_size", workerCfg.BatchSize),
		zap.Duration("poll_interval", workerCfg.PollInterval),
		zap.Int("max_attempts", workerCfg.MaxAttempts),
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("notification worker shutting down")
	cancel()
	time.Sleep(2 * time.Second)
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
