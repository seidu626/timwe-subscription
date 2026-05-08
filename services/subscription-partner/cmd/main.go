package main

import (
	"database/sql"
	"fmt"
	cached "github.com/seidu626/subscription-manager/common/cache"
	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription/internal/handler"
	"github.com/seidu626/subscription-manager/subscription/internal/middleware"
	"github.com/seidu626/subscription-manager/subscription/internal/repository"
	"github.com/seidu626/subscription-manager/subscription/internal/service"
	"github.com/seidu626/subscription-manager/subscription/internal/transport"
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
	cfg := config.InitConfig(logger, ".", []string{"config.yaml", ".env"})
	// Note: Sensitive config values (passwords, secrets) are NOT logged for security
	logger.Info("Configuration loaded",
		zap.String("environment", string(cfg.Application.Environment)),
		zap.Int("port", cfg.Application.Port),
		zap.String("db_host", cfg.Database.Postgresql.Host),
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

	repo := repository.NewSubscriptionRepository(db, redisClient)
	svc := service.NewSubscriptionService(repo, cfg)
	h := handler.NewSubscriptionHandler(svc, cfg)
	acquisitionClient := service.NewAcquisitionClient(logger)
	notificationWebhookHandler := handler.NewNotificationWebhookHandler(logger, svc, acquisitionClient)

	productRepo := repository.NewProductRepository(db, redisClient)
	productService := service.NewProductService(productRepo)
	productHandler := handler.NewProductHandler(productService)

	router := transport.NewRouter(h, productHandler, notificationWebhookHandler)
	handlerWithCORS := middleware.CORSMiddleware(router, cfg.Application.AllowedOrigins)

	log.Printf("Starting subscription service on port: %d...", cfg.Application.Port)
	log.Fatal(fasthttp.ListenAndServe(fmt.Sprintf(":%d", cfg.Application.Port), handlerWithCORS))
}
