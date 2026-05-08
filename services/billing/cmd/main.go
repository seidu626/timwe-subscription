package main

import (
	"database/sql"
	"fmt"
	"github.com/seidu626/subscription-manager/billing/internal/config"
	"github.com/seidu626/subscription-manager/billing/internal/handler"
	"github.com/seidu626/subscription-manager/billing/internal/repository"
	"github.com/seidu626/subscription-manager/billing/internal/service"
	"github.com/seidu626/subscription-manager/billing/internal/transport"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"log"

	_ "github.com/lib/pq"
	"github.com/valyala/fasthttp"
)

func main() {
	conf := viper.GetViper()
	logger, _ := zap.NewDevelopment()
	cfg := config.InitConfig(conf, *logger, ".", &[]string{"config.yml"})
	// Note: Sensitive config values (passwords, secrets) are NOT logged for security
	logger.Info("Configuration loaded",
		zap.String("environment", conf.GetString("ENVIRONMENT")),
		zap.Int("port", cfg.Application.Port),
		zap.String("db_host", cfg.DB.Postgresql.DBHost),
	)

	if conf.GetString("ENVIRONMENT") == config.PRODUCTION {
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

	repo := repository.NewBillingRepository(db)
	svc := service.NewBillingService(repo)
	h := handler.NewBillingHandler(svc)

	router := transport.NewRouter(h)

	log.Printf("Starting billing service on port: %d...", cfg.Application.Port)
	log.Fatal(fasthttp.ListenAndServe(fmt.Sprintf(":%d", cfg.Application.Port), router))
}
