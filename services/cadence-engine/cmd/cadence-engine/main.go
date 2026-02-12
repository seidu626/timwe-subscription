package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/seidu626/subscription-manager/cadence-engine/internal/adminhttp"
	"github.com/seidu626/subscription-manager/cadence-engine/internal/advancer"
	"github.com/seidu626/subscription-manager/cadence-engine/internal/backfill"
	"github.com/seidu626/subscription-manager/cadence-engine/internal/planner"
	"github.com/seidu626/subscription-manager/cadence-engine/internal/repository"
	"github.com/seidu626/subscription-manager/common/config"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	config.InitConfig(logger, ".", []string{"config.yaml", ".env"})

	connStr := config.GetDBConnectionString()
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	repo := repository.NewCadenceRepository(db, logger)

	plannerCfg := loadPlannerConfig()
	advancerCfg := loadAdvancerConfig()
	backfillCfg := loadBackfillConfig()

	plannerWorker := planner.NewPlanner(repo, logger, plannerCfg)
	advancerWorker := advancer.NewAdvancer(repo, logger, advancerCfg)
	backfillWorker := backfill.NewBackfill(repo, logger, backfillCfg)
	adminServer := adminhttp.NewServer(repo, logger, adminhttp.Config{
		Addr: getEnvString("CADENCE_ADMIN_HTTP_ADDR", ":8091"),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := plannerWorker.Run(ctx); err != nil && err != context.Canceled {
			logger.Error("planner worker stopped", zap.Error(err))
		}
	}()

	go func() {
		if err := advancerWorker.Run(ctx); err != nil && err != context.Canceled {
			logger.Error("advancer worker stopped", zap.Error(err))
		}
	}()

	go func() {
		if err := backfillWorker.Run(ctx); err != nil && err != context.Canceled {
			logger.Error("backfill worker stopped", zap.Error(err))
		}
	}()

	go func() {
		if err := adminServer.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("admin http server stopped", zap.Error(err))
		}
	}()

	logger.Info("cadence-engine started",
		zap.Int("planner_batch_size", plannerCfg.BatchSize),
		zap.Duration("planner_poll_interval", plannerCfg.PollInterval),
		zap.Int("advancer_batch_size", advancerCfg.BatchSize),
		zap.Duration("advancer_poll_interval", advancerCfg.PollInterval),
		zap.Int("backfill_batch_size", backfillCfg.BatchSize),
		zap.Duration("backfill_poll_interval", backfillCfg.PollInterval),
		zap.String("admin_http_addr", getEnvString("CADENCE_ADMIN_HTTP_ADDR", ":8091")),
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("cadence-engine shutting down")
	cancel()
	time.Sleep(2 * time.Second)
}

func loadPlannerConfig() planner.PlannerConfig {
	return planner.PlannerConfig{
		BatchSize:        getEnvInt("CADENCE_PLANNER_BATCH_SIZE", 500),
		PollInterval:     getEnvDuration("CADENCE_PLANNER_POLL_INTERVAL", 10*time.Second),
		InflightDuration: getEnvDuration("CADENCE_PLANNER_INFLIGHT", 10*time.Minute),
	}
}

func loadAdvancerConfig() advancer.AdvancerConfig {
	return advancer.AdvancerConfig{
		BatchSize:    getEnvInt("CADENCE_ADVANCER_BATCH_SIZE", 500),
		PollInterval: getEnvDuration("CADENCE_ADVANCER_POLL_INTERVAL", 10*time.Second),
	}
}

func loadBackfillConfig() backfill.BackfillConfig {
	return backfill.BackfillConfig{
		BatchSize:    getEnvInt("CADENCE_BACKFILL_BATCH_SIZE", 1000),
		PollInterval: getEnvDuration("CADENCE_BACKFILL_POLL_INTERVAL", 30*time.Second),
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

func getEnvString(key string, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
