package config

import (
	"bufio"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Environment string

const (
	DEVELOPMENT Environment = "DEVELOPMENT"
	PRODUCTION  Environment = "PRODUCTION"
)

var cfg Config
var doOnce sync.Once

type Config struct {
	Application struct {
		Environment    Environment `mapstructure:"ENVIRONMENT"`
		Port           int         `mapstructure:"PORT"`
		AllowedOrigins []string    `mapstructure:"ALLOWED_ORIGINS"`
		Log            struct {
			Path string `mapstructure:"PATH"`
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
		JwtToken struct {
			Type           string `mapstructure:"TYPE"`
			Expired        string `mapstructure:"EXPIRED"`
			Secret         string `mapstructure:"SECRET"`
			RefreshExpired string `mapstructure:"REFRESH_EXPIRED"`
		} `mapstructure:"JWT_TOKEN"`
	} `mapstructure:"AUTH"`

	DB struct {
		Postgresql struct {
			DBHost     string `mapstructure:"HOST"`
			DBPort     string `mapstructure:"PORT"`
			DBUser     string `mapstructure:"USER"`
			DBPassword string `mapstructure:"PASSWORD"`
			DBName     string `mapstructure:"DB_NAME"`
			SSLMode    string `mapstructure:"SSL_MODE"`
		} `mapstructure:"POSTGRESQL"`
	} `mapstructure:"DB"`

	Cache struct {
		Redis struct {
			Host string `mapstructure:"HOST"`
			Port int    `mapstructure:"PORT"`
			DB   int    `mapstructure:"DB"`
			Pass string `mapstructure:"PASS"`
		} `mapstructure:"REDIS"`
	} `mapstructure:"CACHE"`
}

func InitConfig(logger *zap.Logger, path string, files []string) Config {
	conf := viper.GetViper()
	conf.SetDefault("ENVIRONMENT", "DEVELOPMENT")
	conf.SetDefault("APPLICATION.PORT", 8082)
	conf.SetDefault("CACHE.REDIS.HOST", "localhost")
	conf.SetDefault("CACHE.REDIS.PORT", 6379)
	conf.SetDefault("CACHE.REDIS.DB", 0)
	conf.SetDefault("DB.POSTGRESQL.HOST", "localhost")
	conf.SetDefault("DB.POSTGRESQL.PORT", "5432")
	conf.SetDefault("DB.POSTGRESQL.SSL_MODE", "disable")

	for _, file := range files {
		if strings.HasSuffix(file, ".env") {
			loadDotEnv(logger, filepath.Join(path, file))
		}
	}

	conf.AddConfigPath(path)
	for _, file := range files {
		conf.SetConfigFile(file)
	}

	// Environment variable support
	//
	// Docker Compose (and our docs) use env vars like:
	//   APP_DATABASE_POSTGRESQL_HOST, APP_CACHE_REDIS_HOST, JWT_SECRET
	//
	// Viper does not automatically map nested keys to those names without explicit binding.
	// We bind the canonical env vars and keep compatibility with legacy variants.
	conf.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	bindEnv(logger, conf)
	conf.AutomaticEnv()

	if err := conf.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Info("Using default settings, config file not found", zap.Error(err))
		} else {
			logger.Info("Config file was found, but an error occurred", zap.Error(err))
		}
	}
	doOnce.Do(func() {
		err := conf.Unmarshal(&cfg)
		if err != nil {
			log.Fatalln("cannot unmarshalling config")
		}
	})

	conf.OnConfigChange(func(e fsnotify.Event) {
		logger.Info("Config file changed", zap.String("File", e.Name))
		// Note: Sensitive config values are NOT logged for security
	})
	conf.WatchConfig()

	return cfg
}

func bindEnv(logger *zap.Logger, conf *viper.Viper) {
	mustBindEnv(logger, conf, "DB.POSTGRESQL.HOST", "APP_DATABASE_POSTGRESQL_HOST", "DB_POSTGRESQL_HOST", "DB.POSTGRESQL.HOST")
	mustBindEnv(logger, conf, "DB.POSTGRESQL.PORT", "APP_DATABASE_POSTGRESQL_PORT", "DB_POSTGRESQL_PORT", "DB.POSTGRESQL.PORT")
	mustBindEnv(logger, conf, "DB.POSTGRESQL.USER", "APP_DATABASE_POSTGRESQL_USER", "DB_POSTGRESQL_USER", "DB.POSTGRESQL.USER")
	mustBindEnv(logger, conf, "DB.POSTGRESQL.PASSWORD", "APP_DATABASE_POSTGRESQL_PASSWORD", "DB_POSTGRESQL_PASSWORD", "DB.POSTGRESQL.PASSWORD")
	mustBindEnv(logger, conf, "DB.POSTGRESQL.DB_NAME", "APP_DATABASE_POSTGRESQL_DB_NAME", "DB_POSTGRESQL_DB_NAME", "DB.POSTGRESQL.DB_NAME")
	mustBindEnv(logger, conf, "DB.POSTGRESQL.SSL_MODE", "APP_DATABASE_POSTGRESQL_SSL_MODE", "DB_POSTGRESQL_SSL_MODE", "DB.POSTGRESQL.SSL_MODE")

	mustBindEnv(logger, conf, "CACHE.REDIS.HOST", "APP_CACHE_REDIS_HOST", "CACHE_REDIS_HOST", "CACHE.REDIS.HOST")
	mustBindEnv(logger, conf, "CACHE.REDIS.PORT", "APP_CACHE_REDIS_PORT", "CACHE_REDIS_PORT", "CACHE.REDIS.PORT")
	mustBindEnv(logger, conf, "CACHE.REDIS.DB", "APP_CACHE_REDIS_DB", "CACHE_REDIS_DB", "CACHE.REDIS.DB")
	mustBindEnv(logger, conf, "CACHE.REDIS.PASS", "APP_CACHE_REDIS_PASS", "CACHE_REDIS_PASS", "CACHE.REDIS.PASS")

	// JWT secret is provided via env var `JWT_SECRET` (documented in config.yaml and compose files).
	mustBindEnv(logger, conf, "AUTH.JWT_TOKEN.SECRET", "JWT_SECRET", "AUTH_JWT_TOKEN_SECRET", "AUTH.JWT_TOKEN.SECRET")
}

func loadDotEnv(logger *zap.Logger, path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

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
		value := strings.TrimSpace(parts[1])
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
	if err := scanner.Err(); err != nil {
		logger.Warn("Env file scan error (continuing)", zap.String("path", path), zap.Error(err))
	}
}

func mustBindEnv(logger *zap.Logger, conf *viper.Viper, key string, envs ...string) {
	args := append([]string{key}, envs...)
	if err := conf.BindEnv(args...); err != nil {
		// Do not fail hard; keep defaults/config file behavior.
		logger.Warn("failed to bind env var", zap.String("key", key), zap.Error(err))
	}
}

func GetDBConnectionString(config Config) string {
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		config.DB.Postgresql.DBHost,
		config.DB.Postgresql.DBPort,
		config.DB.Postgresql.DBUser,
		config.DB.Postgresql.DBPassword,
		config.DB.Postgresql.DBName,
		config.DB.Postgresql.SSLMode,
	)
	return connStr
}

func GetRedisOptions(config Config) *redis.Options {
	return &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Cache.Redis.Host, config.Cache.Redis.Port),
		Password: config.Cache.Redis.Pass,
		DB:       config.Cache.Redis.DB,
	}
}
