package main

import (
	"database/sql"
	"fmt"
	"github.com/redis/go-redis/v9"
	"github.com/seidu626/subscription-manager/notification/internal/config"
	"github.com/seidu626/subscription-manager/notification/internal/handler"
	"github.com/seidu626/subscription-manager/notification/internal/middleware"
	"github.com/seidu626/subscription-manager/notification/internal/repository"
	"github.com/seidu626/subscription-manager/notification/internal/service"
	"github.com/seidu626/subscription-manager/notification/internal/transport"
	"go.uber.org/zap"
	"log"

	_ "github.com/lib/pq"
	"github.com/valyala/fasthttp"
)

func main() {
	logger, _ := zap.NewDevelopment()
	defer func() {
		_ = logger.Sync() // flushes buffer, if any
	}()
	cfg := config.InitConfig(logger, ".", []string{"config.yaml"})
	// Note: Sensitive config values (passwords, secrets) are NOT logged for security
	logger.Info("Configuration loaded",
		zap.String("environment", string(cfg.Application.Environment)),
		zap.Int("port", cfg.Application.Port),
		zap.String("db_host", cfg.DB.Postgresql.DBHost),
		zap.String("redis_host", cfg.Cache.Redis.Host),
	)

	if cfg.Application.Environment == config.PRODUCTION {
		logger, _ = zap.NewProduction()
	}
	defer func(logger *zap.Logger) {
		err := logger.Sync()
		if err != nil {

		}
	}(logger)

	// Get the connection string from the config
	connStr := config.GetDBConnectionString(cfg)
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

	redisOptions := config.GetRedisOptions(cfg)
	redisClient := redis.NewClient(redisOptions)

	repo := repository.NewNotificationRepository(db, redisClient)
	svc := service.NewNotificationService(repo)
	h := handler.NewNotificationHandler(svc)

	router := transport.NewRouter(h)
	handlerWithCORS := middleware.CORSMiddleware(router, cfg.Application.AllowedOrigins)

	log.Printf("Starting notification service on port: %d...", cfg.Application.Port)
	log.Fatal(fasthttp.ListenAndServe(fmt.Sprintf(":%d", cfg.Application.Port), handlerWithCORS))
}
