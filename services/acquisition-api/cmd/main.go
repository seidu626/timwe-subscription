package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/handler"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/service"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/transport"
	"github.com/seidu626/subscription-manager/common/config"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// Initialize logger with explicit JSON encoding to stdout
	logConfig := zap.NewProductionConfig()
	logConfig.OutputPaths = []string{"stdout"}
	logConfig.ErrorOutputPaths = []string{"stderr"}
	logConfig.Encoding = "json"
	logConfig.EncoderConfig.TimeKey = "ts"
	logConfig.EncoderConfig.EncodeTime = zapcore.EpochTimeEncoder

	logger, err := logConfig.Build()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() { _ = logger.Sync() }()

	// Load configuration
	cfg := config.InitConfig(logger, ".", []string{"config.yaml"})

	// Connect to database
	logger.Info("Connecting to database",
		zap.String("host", cfg.Database.Postgresql.Host),
		zap.String("port", cfg.Database.Postgresql.Port),
		zap.String("user", cfg.Database.Postgresql.User),
		zap.String("db_name", cfg.Database.Postgresql.DBName),
		zap.String("ssl_mode", cfg.Database.Postgresql.SSLMode),
	)
	connStr := config.GetDBConnectionString()
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	logger.Info("Database connection established")

	// Initialize repositories
	campaignRepo := repository.NewCampaignRepository(db, logger)
	transactionRepo := repository.NewTransactionRepository(db, logger)
	postbackRepo := repository.NewPostbackRepository(db, logger)
	landingEventRepo := repository.NewLandingEventRepository(db, logger)
	reportsRepo := repository.NewReportsRepository(db, logger)
	outboundClickRepo := repository.NewOutboundClickRepository(db, logger)

	// Initialize TIMWE client with configuration
	timweConfig := buildTIMWEConfig(cfg)

	// Validate secrets are configured (fatal if missing in production)
	if cfg.Application.Environment != "DEVELOPMENT" {
		validateConfig(timweConfig, logger)
	} else if timweConfig.APIKey == "" || timweConfig.PSK == "" {
		logger.Warn("TIMWE secrets not configured - set TIMWE_API_KEY and TIMWE_PSK environment variables")
	}

	// SECURITY: Fail fast if HE simulation is enabled in production
	heSimEnabled := os.Getenv("HE_SIMULATION_ENABLED") == "true"
	if heSimEnabled && cfg.Application.Environment == "PRODUCTION" {
		logger.Fatal("SECURITY VIOLATION: HE_SIMULATION_ENABLED=true is not allowed in production",
			zap.String("environment", string(cfg.Application.Environment)),
			zap.Bool("he_simulation_enabled", heSimEnabled),
		)
	}
	if heSimEnabled {
		logger.Warn("HE Simulation mode is ENABLED - this should only be used in staging/local environments",
			zap.String("environment", string(cfg.Application.Environment)),
		)
	}

	timweClient := service.NewTIMWEClientWithConfig(timweConfig, logger)
	logger.Info("TIMWE client initialized",
		zap.String("base_url", timweConfig.BaseURL),
		zap.String("partner_role_id", timweConfig.PartnerRoleID),
	)

	// Initialize services
	providerRegistry := service.NewProviderRegistry(logger)
	registerAdProviders(providerRegistry, logger)
	campaignService := service.NewCampaignService(campaignRepo, logger)
	transactionService := service.NewTransactionService(
		transactionRepo,
		campaignRepo,
		postbackRepo,
		providerRegistry,
		timweClient,
		logger,
	)

	// Initialize handlers
	campaignHandler := handler.NewCampaignHandler(campaignService, logger)
	transactionHandler := handler.NewTransactionHandler(transactionService, logger)
	callbackHandler := handler.NewCallbackHandler(transactionRepo, postbackRepo, providerRegistry, logger)
	internalHandler := handler.NewInternalHandler(transactionService, logger)
	analyticsHandler := handler.NewAnalyticsHandler(landingEventRepo, logger)
	reportsHandler := handler.NewReportsHandler(reportsRepo, logger)
	postbackAdminHandler := handler.NewPostbackAdminHandler(postbackRepo, logger)
	transactionAdminHandler := handler.NewTransactionAdminHandler(transactionRepo, logger)

	// Initialize click-out handler (optional, configured via environment)
	var clickOutHandler *handler.ClickOutHandler
	clickOutConfig := buildClickOutConfig()
	if len(clickOutConfig.Destinations) > 0 {
		clickOutHandler = handler.NewClickOutHandler(outboundClickRepo, clickOutConfig, logger)
		logger.Info("Click-out handler initialized", zap.Int("destinations", len(clickOutConfig.Destinations)))
	} else {
		logger.Info("Click-out handler not configured (no destinations)")
	}

	// Initialize HE bootstrap handler (for HTTP-only Header Enrichment capture)
	var heBootstrapHandler *handler.HEBootstrapHandler
	heBootstrapConfig := buildHEBootstrapConfig(logger)
	if heBootstrapConfig.RedisClient != nil {
		heBootstrapHandler = handler.NewHEBootstrapHandler(heBootstrapConfig, logger)
		logger.Info("HE bootstrap handler initialized",
			zap.String("https_host", heBootstrapConfig.HTTPSHost),
			zap.Duration("token_ttl", heBootstrapConfig.TokenTTL),
		)
	} else {
		logger.Info("HE bootstrap handler not configured (Redis not available)")
	}

	// Create router
	router := transport.NewRouter(campaignHandler, transactionHandler, callbackHandler, internalHandler, analyticsHandler, reportsHandler, postbackAdminHandler, transactionAdminHandler, clickOutHandler, heBootstrapHandler)

	// Create server
	server := &fasthttp.Server{
		Handler:            router,
		ReadTimeout:        cfg.Application.HTTP.ReadTimeout,
		WriteTimeout:       cfg.Application.HTTP.WriteTimeout,
		IdleTimeout:        cfg.Application.HTTP.IdleTimeout,
		MaxRequestBodySize: cfg.Application.HTTP.MaxRequestBodyMB * 1024 * 1024,
		Concurrency:        cfg.Application.HTTP.Concurrency,
		ReduceMemoryUsage:  true,
	}

	// Set defaults
	if server.ReadTimeout == 0 {
		server.ReadTimeout = 60 * time.Second
	}
	if server.WriteTimeout == 0 {
		server.WriteTimeout = 60 * time.Second
	}
	if server.IdleTimeout == 0 {
		server.IdleTimeout = 120 * time.Second
	}
	if server.MaxRequestBodySize == 0 {
		server.MaxRequestBodySize = 16 * 1024 * 1024
	}

	port := cfg.Application.Port
	if port == 0 {
		port = 8084 // Default port for acquisition-api
	}

	logger.Info("Starting acquisition API service", zap.Int("port", port))

	// Set up signal handling for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(fmt.Sprintf(":%d", port)); err != nil {
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

// buildTIMWEConfig creates TIMWEConfig from the common config and environment variables
func buildTIMWEConfig(cfg *config.Config) *service.TIMWEConfig {
	timweCfg := service.DefaultTIMWEConfig()

	// Override with values from config
	if cfg.Application.TIMWE.BaseURL != "" {
		timweCfg.BaseURL = cfg.Application.TIMWE.BaseURL
	}

	// Load API key from config or environment variable (env takes precedence)
	if envKey := os.Getenv("TIMWE_API_KEY"); envKey != "" {
		timweCfg.APIKey = envKey
	} else if cfg.Application.TIMWE.APIKey != "" && !isEnvVarReference(cfg.Application.TIMWE.APIKey) {
		timweCfg.APIKey = cfg.Application.TIMWE.APIKey
	}

	// Load PSK from config or environment variable (env takes precedence)
	if envPsk := os.Getenv("TIMWE_PSK"); envPsk != "" {
		timweCfg.PSK = envPsk
	} else if cfg.Application.TIMWE.Psk != "" && !isEnvVarReference(cfg.Application.TIMWE.Psk) {
		timweCfg.PSK = cfg.Application.TIMWE.Psk
	}

	if cfg.Application.TIMWE.PartnerRoleID != "" {
		timweCfg.PartnerRoleID = cfg.Application.TIMWE.PartnerRoleID
	}
	if cfg.Application.TIMWE.PartnerServiceID != "" {
		timweCfg.PartnerServiceID = cfg.Application.TIMWE.PartnerServiceID
	}
	if cfg.Application.TIMWE.MCC != "" {
		timweCfg.MCC = cfg.Application.TIMWE.MCC
	}
	if cfg.Application.TIMWE.MNC != "" {
		timweCfg.MNC = cfg.Application.TIMWE.MNC
	}
	if cfg.Application.TIMWE.Timeout > 0 {
		timweCfg.Timeout = cfg.Application.TIMWE.Timeout
	}
	if cfg.Application.TIMWE.MaxConnections > 0 {
		timweCfg.MaxConnections = cfg.Application.TIMWE.MaxConnections
	}

	// Circuit breaker settings
	if cfg.Application.TIMWE.CBMaxRequests > 0 {
		timweCfg.CBMaxRequests = uint32(cfg.Application.TIMWE.CBMaxRequests)
	}
	if cfg.Application.TIMWE.CBTimeout > 0 {
		timweCfg.CBTimeout = cfg.Application.TIMWE.CBTimeout
	}
	if cfg.Application.TIMWE.CBInterval > 0 {
		timweCfg.CBInterval = cfg.Application.TIMWE.CBInterval
	}
	if cfg.Application.TIMWE.CBMinRequests > 0 {
		timweCfg.CBMinRequests = uint32(cfg.Application.TIMWE.CBMinRequests)
	}
	if cfg.Application.TIMWE.CBFailureRateThreshold > 0 {
		timweCfg.CBFailureRateThreshold = cfg.Application.TIMWE.CBFailureRateThreshold
	}
	if cfg.Application.TIMWE.CBConsecutiveFailures > 0 {
		timweCfg.CBConsecutiveFailures = uint32(cfg.Application.TIMWE.CBConsecutiveFailures)
	}

	// Retry settings (using charge retry config fields)
	if cfg.Application.TIMWE.ChargeRetryBaseDelay > 0 {
		timweCfg.RetryBaseDelay = cfg.Application.TIMWE.ChargeRetryBaseDelay
	}
	if cfg.Application.TIMWE.ChargeRetryMaxDelay > 0 {
		timweCfg.RetryMaxDelay = cfg.Application.TIMWE.ChargeRetryMaxDelay
	}

	return timweCfg
}

// isEnvVarReference checks if a value is an unresolved environment variable reference
func isEnvVarReference(val string) bool {
	return len(val) > 2 && val[0] == '$' && val[1] == '{'
}

// validateConfig validates required configuration and secrets are set
func validateConfig(timweCfg *service.TIMWEConfig, logger *zap.Logger) {
	missing := []string{}

	if timweCfg.APIKey == "" {
		missing = append(missing, "TIMWE_API_KEY")
	}
	if timweCfg.PSK == "" {
		missing = append(missing, "TIMWE_PSK")
	}

	if len(missing) > 0 {
		logger.Fatal("Required secrets not configured - set environment variables",
			zap.Strings("missing", missing))
	}
}

// buildClickOutConfig creates ClickOutConfig from environment variables
// Environment variables:
//   - CLICKOUT_DESTINATIONS_JSON: JSON map of dest_key -> destination config.
//     Example:
//     {
//     "partnerA": {"base_url":"https://example.com/click","click_id_param":"click_id","passthrough_params":["sub1","sub2"]},
//     "landing_web": {"base_url":"https://landing.example.com/lp/test","click_id_param":"click_id","passthrough_params":["utm_source"]}
//     }
//   - CLICKOUT_RATE_LIMIT: Max clicks per IP per hour (default: 100)
//   - CLICKOUT_COOKIE_DOMAIN: Domain for click_id cookie (optional)
//   - CLICKOUT_COOKIE_SECURE: Set Secure flag on cookie (default: true)
func buildClickOutConfig() *handler.ClickOutConfig {
	config := &handler.ClickOutConfig{
		Destinations:          make(map[string]handler.DestinationConfig),
		DefaultClickIDParam:   "click_id",
		RateLimitPerIPPerHour: 100,
		CookieSecure:          true,
	}

	// Destinations are configured via JSON to keep the platform partner-agnostic.
	if raw := os.Getenv("CLICKOUT_DESTINATIONS_JSON"); raw != "" {
		type dest struct {
			BaseURL           string   `json:"base_url"`
			ClickIDParam      string   `json:"click_id_param"`
			PassthroughParams []string `json:"passthrough_params"`
			AllowedPartners   []string `json:"allowed_partners"`
		}
		m := map[string]dest{}
		if err := json.Unmarshal([]byte(raw), &m); err == nil {
			for key, d := range m {
				if d.BaseURL == "" {
					continue
				}
				config.Destinations[key] = handler.DestinationConfig{
					BaseURL:           d.BaseURL,
					ClickIDParam:      d.ClickIDParam,
					PassthroughParams: d.PassthroughParams,
					AllowedPartners:   d.AllowedPartners,
				}
			}
		}
	}

	// Rate limit override
	if rateLimit := os.Getenv("CLICKOUT_RATE_LIMIT"); rateLimit != "" {
		var limit int
		if _, err := fmt.Sscanf(rateLimit, "%d", &limit); err == nil && limit > 0 {
			config.RateLimitPerIPPerHour = limit
		}
	}

	// Cookie domain
	if domain := os.Getenv("CLICKOUT_COOKIE_DOMAIN"); domain != "" {
		config.CookieDomain = domain
	}

	// Cookie secure flag
	if secure := os.Getenv("CLICKOUT_COOKIE_SECURE"); secure == "false" {
		config.CookieSecure = false
	}

	return config
}

// registerAdProviders registers ad providers based on environment configuration.
// Environment variables:
//   - AD_PROVIDERS: comma-separated list of provider names to enable (default: "generic")
func registerAdProviders(reg *service.ProviderRegistry, logger *zap.Logger) {
	enabled := os.Getenv("AD_PROVIDERS")
	if strings.TrimSpace(enabled) == "" {
		enabled = "generic"
	}
	for _, name := range strings.Split(enabled, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		switch name {
		case "generic":
			reg.Register(service.NewGenericProvider(logger))
		case "mobplus":
			reg.Register(service.NewMobplusProvider(logger))
		default:
			logger.Warn("Unknown ad provider requested; skipping", zap.String("provider", name))
		}
	}
}

// buildHEBootstrapConfig creates HEBootstrapConfig from environment variables
// Environment variables:
//   - HE_BOOTSTRAP_TOKEN_TTL: Token TTL in seconds (default: 60)
//   - HE_BOOTSTRAP_TOKEN_SECRET: Optional secret for additional token security
//   - HE_BOOTSTRAP_HTTPS_HOST: HTTPS host to redirect to (default: landing.nouveauricheglobalgroup.com)
//   - HE_TRUSTED_PROXY_CIDRS: Comma-separated list of trusted operator proxy CIDRs
//   - REDIS_HOST, REDIS_PORT, REDIS_PASSWORD: Redis connection settings
func buildHEBootstrapConfig(logger *zap.Logger) *handler.HEBootstrapConfig {
	cfg := handler.DefaultHEBootstrapConfig()

	// Initialize Redis client
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = os.Getenv("APP_CACHE_REDIS_HOST")
	}
	if redisHost == "" {
		// Try to use common config
		opts := config.GetRedisOptions()
		if opts != nil && opts.Addr != ":0" && opts.Addr != "" {
			cfg.RedisClient = redis.NewClient(opts)
		}
	} else {
		redisPort := os.Getenv("REDIS_PORT")
		if redisPort == "" {
			redisPort = os.Getenv("APP_CACHE_REDIS_PORT")
		}
		if redisPort == "" {
			redisPort = "6379"
		}
		redisPassword := os.Getenv("REDIS_PASSWORD")
		if redisPassword == "" {
			redisPassword = os.Getenv("APP_CACHE_REDIS_PASSWORD")
		}

		cfg.RedisClient = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
			Password: redisPassword,
			DB:       0,
		})
	}

	// Test Redis connection if client was created
	if cfg.RedisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := cfg.RedisClient.Ping(ctx).Err(); err != nil {
			logger.Warn("Failed to connect to Redis for HE bootstrap, handler will be disabled",
				zap.Error(err),
			)
			cfg.RedisClient = nil
		} else {
			logger.Info("Redis connection established for HE bootstrap")
		}
	}

	// Create HE context middleware for identity extraction
	heContextConfig := handler.DefaultHEContextConfig()
	cfg.HEMiddleware = handler.NewHEContextMiddleware(heContextConfig, logger)

	return cfg
}
