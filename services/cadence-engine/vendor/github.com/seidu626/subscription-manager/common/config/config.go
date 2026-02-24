package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Environment string

const (
	DEVELOPMENT Environment = "DEVELOPMENT"
	PRODUCTION              = "PRODUCTION"
)

type Config struct {
	Application struct {
		Environment    Environment         `mapstructure:"ENVIRONMENT"`
		Port           int                 `mapstructure:"PORT"`
		AllowedOrigins []string            `mapstructure:"ALLOWED_ORIGINS"`
		TelcoPrefixes  map[string][]string `mapstructure:"TELCO_PREFIXES"`
		TIMWE          struct {
			Host              string        `mapstructure:"HOST"`
			BaseURL           string        `mapstructure:"BASE_URL"`
			APIKey            string        `mapstructure:"API_KEY"`
			MTAPIKey          string        `mapstructure:"MT_API_KEY"`
			Psk               string        `mapstructure:"PSK"`
			PartnerServiceID  string        `mapstructure:"PARTNER_SERVICE_ID"`
			PartnerRoleID     string        `mapstructure:"PARTNER_ROLE_ID"`
			Realm             string        `mapstructure:"REALM"`
			AuthenticationKey string        `mapstructure:"AUTHENTICATION_KEY"`
			MCC               string        `mapstructure:"MCC"`
			MNC               string        `mapstructure:"MNC"`
			Timeout           time.Duration `mapstructure:"TIMEOUT"`
			MaxConnections    int           `mapstructure:"MAX_CONNECTIONS"`
			// Charge retry settings
			ChargeRetryMaxDuration time.Duration `mapstructure:"CHARGE_RETRY_MAX_DURATION"`
			ChargeRetryBaseDelay   time.Duration `mapstructure:"CHARGE_RETRY_BASE_DELAY"`
			ChargeRetryMaxDelay    time.Duration `mapstructure:"CHARGE_RETRY_MAX_DELAY"`
			// Circuit breaker settings
			CBMaxRequests          int           `mapstructure:"CB_MAX_REQUESTS"`
			CBTimeout              time.Duration `mapstructure:"CB_TIMEOUT"`
			CBInterval             time.Duration `mapstructure:"CB_INTERVAL"`
			CBMinRequests          int           `mapstructure:"CB_MIN_REQUESTS"`
			CBFailureRateThreshold float64       `mapstructure:"CB_FAILURE_RATE_THRESHOLD"`
			CBConsecutiveFailures  int           `mapstructure:"CB_CONSECUTIVE_FAILURES"`
		} `mapstructure:"TIMWE_MA"`
		// New HTTP server tunables
		HTTP struct {
			ReadTimeout      time.Duration `mapstructure:"READ_TIMEOUT"`
			WriteTimeout     time.Duration `mapstructure:"WRITE_TIMEOUT"`
			IdleTimeout      time.Duration `mapstructure:"IDLE_TIMEOUT"`
			MaxRequestBodyMB int           `mapstructure:"MAX_REQUEST_BODY_MB"`
			Concurrency      int           `mapstructure:"CONCURRENCY"`
		} `mapstructure:"HTTP"`
		// Batch processing controls
		Batch struct {
			MaxWorkersPerJob    int `mapstructure:"MAX_WORKERS_PER_JOB"`
			MaxConcurrentOptins int `mapstructure:"MAX_CONCURRENT_OPTINS"`
			TargetQPS           int `mapstructure:"TARGET_QPS"`
		} `mapstructure:"BATCH"`
		// MSISDN Generator Configuration
		MSISDNGenerator struct {
			Enabled            bool          `mapstructure:"ENABLED"`
			BatchSize          int           `mapstructure:"BATCH_SIZE"`
			MaxConcurrent      int           `mapstructure:"MAX_CONCURRENT"`
			MaxMSISDNCount     int           `mapstructure:"MAX_MSISDN_COUNT"`
			CacheEnabled       bool          `mapstructure:"CACHE_ENABLED"`
			BloomFilterEnabled bool          `mapstructure:"BLOOM_FILTER_ENABLED"`
			FalsePositiveRate  float64       `mapstructure:"FALSE_POSITIVE_RATE"`
			ValidationTimeout  time.Duration `mapstructure:"VALIDATION_TIMEOUT"`
			GenerationTimeout  time.Duration `mapstructure:"GENERATION_TIMEOUT"`
			WorkerPoolSize     int           `mapstructure:"WORKER_POOL_SIZE"`
			ChannelBufferSize  int           `mapstructure:"CHANNEL_BUFFER_SIZE"`
			FallbackToDatabase bool          `mapstructure:"FALLBACK_TO_DATABASE"`
			MaxRetryAttempts   int           `mapstructure:"MAX_RETRY_ATTEMPTS"`
			PreloadEnabled     bool          `mapstructure:"PRELOAD_ENABLED"`
			PreloadBatchSize   int           `mapstructure:"PRELOAD_BATCH_SIZE"`
		} `mapstructure:"MSISDN_GENERATOR"`
		// MSISDN validation settings
		MSISDNValidation struct {
			CacheExpiry             time.Duration       `mapstructure:"CACHE_EXPIRY"`
			EnablePrefixValidation  bool                `mapstructure:"ENABLE_PREFIX_VALIDATION"`
			EnableExcludedUserCheck bool                `mapstructure:"ENABLE_EXCLUDED_USER_CHECK"`
			EnableInvalidLogCheck   bool                `mapstructure:"ENABLE_INVALID_LOG_CHECK"`
			MaxValidationErrors     int                 `mapstructure:"MAX_VALIDATION_ERRORS"`
			TelcoPrefixes           map[string][]string `mapstructure:"TELCO_PREFIXES"`
		} `mapstructure:"MSISDN_VALIDATION"`
		// Network resilience settings
		NetworkResilience struct {
			MaxRetries              int           `mapstructure:"MAX_RETRIES"`
			BaseRetryDelay          time.Duration `mapstructure:"BASE_RETRY_DELAY"`
			MaxRetryDelay           time.Duration `mapstructure:"MAX_RETRY_DELAY"`
			ConnectionTimeout       time.Duration `mapstructure:"CONNECTION_TIMEOUT"`
			ReadTimeout             time.Duration `mapstructure:"READ_TIMEOUT"`
			WriteTimeout            time.Duration `mapstructure:"WRITE_TIMEOUT"`
			MaxConnsPerHost         int           `mapstructure:"MAX_CONNS_PER_HOST"`
			MaxIdleConnDuration     time.Duration `mapstructure:"MAX_IDLE_CONN_DURATION"`
			CircuitBreakerThreshold int           `mapstructure:"CIRCUIT_BREAKER_THRESHOLD"`
			CircuitBreakerTimeout   time.Duration `mapstructure:"CIRCUIT_BREAKER_TIMEOUT"`
			JitterEnabled           bool          `mapstructure:"JITTER_ENABLED"`
		} `mapstructure:"NETWORK_RESILIENCE"`
		// Enhanced monitoring settings
		EnhancedMonitoring struct {
			EnableAutomatedRecovery bool          `mapstructure:"ENABLE_AUTOMATED_RECOVERY"`
			RecoveryCooldown        time.Duration `mapstructure:"RECOVERY_COOLDOWN"`
			MaxRecoveryAttempts     int           `mapstructure:"MAX_RECOVERY_ATTEMPTS"`
			HealthCheckInterval     time.Duration `mapstructure:"HEALTH_CHECK_INTERVAL"`
			AlertCooldown           time.Duration `mapstructure:"ALERT_COOLDOWN"`
			EnableRealTimeMetrics   bool          `mapstructure:"ENABLE_REAL_TIME_METRICS"`
		} `mapstructure:"ENHANCED_MONITORING"`
		Log struct {
			Path    string `mapstructure:"PATH"`
			Rolling struct {
				Enabled           bool `mapstructure:"ENABLED"`
				MaxSize           int  `mapstructure:"MAX_SIZE"`
				MaxAge            int  `mapstructure:"MAX_AGE"`
				MaxBackups        int  `mapstructure:"MAX_BACKUPS"`
				Compress          bool `mapstructure:"COMPRESS"`
				CompressThreshold int  `mapstructure:"COMPRESS_THRESHOLD"`
				LocalTime         bool `mapstructure:"LOCAL_TIME"`
			} `mapstructure:"ROLLING"`
		}
		Key struct {
			Default string `mapstructure:"DEFAULT"`
			Rsa     struct {
				Public  string `mapstructure:"PUBLIC"`
				Private string `mapstructure:"PRIVATE"`
			}
		}
		Graceful struct {
			MaxSecond time.Duration `mapstructure:"MAX_SECOND"`
		} `mapstructure:"GRACEFUL"`
	} `mapstructure:"APPLICATION"`
	Auth struct {
		Domain       string
		ClientID     string
		ClientSecret string
		Audience     string
		JwtToken     struct {
			Type           string `mapstructure:"TYPE"`
			Expired        string `mapstructure:"EXPIRED"`
			Secret         string `mapstructure:"SECRET"`
			RefreshExpired string `mapstructure:"REFRESH_EXPIRED"`
		} `mapstructure:"JWT_TOKEN"`
	} `mapstructure:"AUTH"`
	Database struct {
		Postgresql struct {
			Host     string `mapstructure:"HOST"`
			Port     string `mapstructure:"PORT"`
			User     string `mapstructure:"USER"`
			Password string `mapstructure:"PASSWORD"`
			DBName   string `mapstructure:"DB_NAME"`
			SSLMode  string `mapstructure:"SSL_MODE"`
			// Connection pool settings
			MaxOpenConns    int           `mapstructure:"MAX_OPEN_CONNS"`
			MaxIdleConns    int           `mapstructure:"MAX_IDLE_CONNS"`
			ConnMaxLifetime time.Duration `mapstructure:"CONN_MAX_LIFETIME"`
			// Enhanced database timeout settings
			QueryTimeout       time.Duration `mapstructure:"QUERY_TIMEOUT"`
			ConnectionTimeout  time.Duration `mapstructure:"CONNECTION_TIMEOUT"`
			TransactionTimeout time.Duration `mapstructure:"TRANSACTION_TIMEOUT"`
			PoolTimeout        time.Duration `mapstructure:"POOL_TIMEOUT"`
		} `mapstructure:"POSTGRESQL"`
	} `mapstructure:"DATABASE"`
	Cache struct {
		Redis struct {
			Host string `mapstructure:"HOST"`
			Port int    `mapstructure:"PORT"`
			DB   int    `mapstructure:"DB"`
			Pass string `mapstructure:"PASS"`
		}
	} `mapstructure:"CACHE"`
	// DynamicConfigs holds configurations registered by external services
	DynamicConfigs map[string]interface{}
}

// Global configuration pointer and mutex
var (
	cfg      *Config
	cfgMutex sync.RWMutex
)

// Registry to hold configuration structs registered by external packages
var (
	registry = make(map[string]interface{})
	regMutex = sync.Mutex{}
)

// RegisterConfig allows external packages to register their configuration struct with a key.
func RegisterConfig(key string, configStruct interface{}) {
	regMutex.Lock()
	defer regMutex.Unlock()
	registry[key] = configStruct
}

// GetDynamicConfig returns the registered configuration for the given key.
func GetDynamicConfig(key string) (interface{}, bool) {
	cfgMutex.RLock()
	defer cfgMutex.RUnlock()
	cfgStruct, exists := cfg.DynamicConfigs[key]
	return cfgStruct, exists
}

// GetDynamicConfigTyped returns the registered configuration for the given key with type assertion.
func GetDynamicConfigTyped[T any](key string) (*T, bool) {
	cfgMutex.RLock()
	defer cfgMutex.RUnlock()
	cfgStruct, exists := cfg.DynamicConfigs[key]
	if !exists {
		return nil, false
	}
	typedCfg, ok := cfgStruct.(*T)
	return typedCfg, ok
}

// InitConfig initializes the configuration by loading config files and setting up watchers.
func InitConfig(logger *zap.Logger, path string, files []string) *Config {
	v := viper.New()
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.SetConfigType("yaml") // or "json", depending on your config files
	cfg = &Config{}
	cfg.DynamicConfigs = make(map[string]interface{})

	// Auto-load .env from common locations (project root, current dir, parent dirs)
	autoLoadDotEnv(logger)

	// Explicitly bind database environment variables (required for nested struct unmarshaling)
	_ = v.BindEnv("DATABASE.POSTGRESQL.HOST", "APP_DATABASE_POSTGRESQL_HOST")
	_ = v.BindEnv("DATABASE.POSTGRESQL.PORT", "APP_DATABASE_POSTGRESQL_PORT")
	_ = v.BindEnv("DATABASE.POSTGRESQL.USER", "APP_DATABASE_POSTGRESQL_USER")
	_ = v.BindEnv("DATABASE.POSTGRESQL.PASSWORD", "APP_DATABASE_POSTGRESQL_PASSWORD")
	_ = v.BindEnv("DATABASE.POSTGRESQL.DB_NAME", "APP_DATABASE_POSTGRESQL_DB_NAME")
	_ = v.BindEnv("DATABASE.POSTGRESQL.SSL_MODE", "APP_DATABASE_POSTGRESQL_SSL_MODE")

	// Bind cache/Redis environment variables
	_ = v.BindEnv("CACHE.REDIS.HOST", "APP_CACHE_REDIS_HOST")
	_ = v.BindEnv("CACHE.REDIS.PORT", "APP_CACHE_REDIS_PORT")

	// Bind auth environment variables
	_ = v.BindEnv("AUTH.JWT_TOKEN.SECRET", "JWT_SECRET")

	// Bind TIMWE environment variables for backward compatibility across services.
	_ = v.BindEnv("APPLICATION.TIMWE_MA.API_KEY", "APP_APPLICATION_TIMWE_MA_API_KEY", "TIMWE_API_KEY")
	_ = v.BindEnv("APPLICATION.TIMWE_MA.PSK", "APP_APPLICATION_TIMWE_MA_PSK", "TIMWE_PSK")
	_ = v.BindEnv("APPLICATION.TIMWE_MA.PARTNER_SERVICE_ID", "APP_APPLICATION_TIMWE_MA_PARTNER_SERVICE_ID", "TIMWE_PARTNER_SERVICE_ID")
	_ = v.BindEnv("APPLICATION.TIMWE_MA.AUTHENTICATION_KEY", "APP_APPLICATION_TIMWE_MA_AUTHENTICATION_KEY", "TIMWE_AUTHENTICATION_KEY")

	// Load configuration files (YAML/JSON). For .env, load as key=value not YAML.
	for _, file := range files {
		if strings.HasSuffix(file, ".env") {
			f, err := os.Open(fmt.Sprintf("%s/%s", path, file))
			if err != nil {
				logger.Warn("Env file load error (continuing)", zap.Error(err))
				continue
			}
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) != 2 {
					continue
				}
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				_ = os.Setenv(key, val)
			}
			_ = f.Close()
			if err := scannerErr(scanner); err != nil {
				logger.Warn("Env file scan error (continuing)", zap.Error(err))
			}
			continue
		}

		v.SetConfigFile(fmt.Sprintf("%s/%s", path, file))
		if err := v.MergeInConfig(); err != nil {
			logger.Warn("Config file load error (continuing)", zap.Error(err))
		}
	}

	// Unmarshal to config struct
	if err := v.Unmarshal(cfg); err != nil {
		logger.Fatal("Error unmarshalling config", zap.Error(err))
	}

	// Unmarshal registered configurations
	regMutex.Lock()
	for key, cfgStruct := range registry {
		if err := v.UnmarshalKey(key, cfgStruct); err != nil {
			logger.Error("Error unmarshalling registered config", zap.String("key", key), zap.Error(err))
		} else {
			cfg.DynamicConfigs[key] = cfgStruct
		}
	}
	regMutex.Unlock()

	// Watch for changes
	v.OnConfigChange(func(e fsnotify.Event) {
		logger.Info("Config file changed", zap.String("file", e.Name))
		newCfg := &Config{}
		if strings.HasSuffix(e.Name, ".env") {
			// Reload .env on change
			f, err := os.Open(e.Name)
			if err == nil {
				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					line := strings.TrimSpace(scanner.Text())
					if line == "" || strings.HasPrefix(line, "#") {
						continue
					}
					parts := strings.SplitN(line, "=", 2)
					if len(parts) != 2 {
						continue
					}
					key := strings.TrimSpace(parts[0])
					val := strings.TrimSpace(parts[1])
					_ = os.Setenv(key, val)
				}
				_ = f.Close()
				if err := scannerErr(scanner); err != nil {
					logger.Warn("Env file scan error (continuing)", zap.Error(err))
				}
			}
		}
		if err := v.Unmarshal(newCfg); err != nil {
			logger.Error("Error reloading config", zap.Error(err))
			return
		}

		// Update the global config in a thread-safe manner
		cfgMutex.Lock()
		*cfg = *newCfg
		cfgMutex.Unlock()

		// Reload registered configurations
		regMutex.Lock()
		for key, cfgStruct := range registry {
			if err := v.UnmarshalKey(key, cfgStruct); err != nil {
				logger.Error("Error reloading registered config", zap.String("key", key), zap.Error(err))
			} else {
				cfgMutex.Lock()
				cfg.DynamicConfigs[key] = cfgStruct
				cfgMutex.Unlock()
				logger.Info("Dynamic config reloaded", zap.String("key", key))
			}
		}
		regMutex.Unlock()
	})
	v.WatchConfig()

	return cfg
}

// scannerErr returns the error from a scanner (workaround to keep file-local)
func scannerErr(s *bufio.Scanner) error { return s.Err() }

// autoLoadDotEnv searches for .env files in common locations and loads them into os environment.
// Search order (first found wins): current dir, parent dirs up to 5 levels, common project paths.
func autoLoadDotEnv(logger *zap.Logger) {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		logger.Debug("Could not get working directory for .env auto-load", zap.Error(err))
		return
	}

	// Paths to search for .env (relative to cwd or absolute)
	searchPaths := []string{
		".env",           // Current directory
		"../.env",        // Parent (services/<name>/ -> services/)
		"../../.env",     // Grandparent (services/<name>/ -> project root)
		"../../../.env",  // Great-grandparent
	}

	for _, relPath := range searchPaths {
		envPath := relPath
		if !strings.HasPrefix(relPath, "/") {
			envPath = fmt.Sprintf("%s/%s", cwd, relPath)
		}

		if _, statErr := os.Stat(envPath); statErr == nil {
			if loadErr := loadEnvFile(envPath); loadErr != nil {
				logger.Debug("Failed to load .env file", zap.String("path", envPath), zap.Error(loadErr))
			} else {
				logger.Debug("Auto-loaded .env file", zap.String("path", envPath))
			}
			return // Stop after first successful load
		}
	}

	logger.Debug("No .env file found in common locations")
}

// loadEnvFile reads a .env file and sets environment variables.
func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Only set if not already set (allow explicit env vars to override .env)
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
	return scanner.Err()
}

// GetDBConnectionString constructs the PostgreSQL connection string from the config.
func GetDBConnectionString() string {
	cfgMutex.RLock()
	defer cfgMutex.RUnlock()
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Postgresql.Host,
		cfg.Database.Postgresql.Port,
		cfg.Database.Postgresql.User,
		cfg.Database.Postgresql.Password,
		cfg.Database.Postgresql.DBName,
		cfg.Database.Postgresql.SSLMode,
	)
	return connStr
}

// GetRedisOptions constructs the Redis options from the config.
func GetRedisOptions() *redis.Options {
	cfgMutex.RLock()
	defer cfgMutex.RUnlock()
	return &redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Cache.Redis.Host, cfg.Cache.Redis.Port),
		Password:     cfg.Cache.Redis.Pass,
		DB:           cfg.Cache.Redis.DB,
		DialTimeout:  5 * time.Second, // Timeout for establishing connection
		ReadTimeout:  3 * time.Second, // Timeout for read operations
		WriteTimeout: 3 * time.Second, // Timeout for write operations
		PoolSize:     10,              // Maximum number of connections in the pool
		MinIdleConns: 5,               // Minimum number of idle connections
		MaxRetries:   3,               // Maximum number of retries for failed commands
	}
}
