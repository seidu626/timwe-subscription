package config

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Environment string

const (
	DEVELOPMENT Environment = "DEVELOPMENT"
	PRODUCTION              = "PRODUCTION"
)

var cfg Config
var doOnce sync.Once

type Config struct {
	Application struct {
		Port int `mapstructure:"PORT"`
		Log  struct {
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
		}
	} `mapstructure:"CACHE"`
}

func InitConfig(conf *viper.Viper, logger zap.Logger, path string, files *[]string) Config {
	conf.SetDefault("ENVIRONMENT", "DEVELOPMENT")
	conf.SetDefault("REDIS_HOST", "redis")
	conf.SetDefault("REDIS_PORT", "6379")
	conf.SetDefault("REDIS_PASSWORD", "")
	conf.SetDefault("POLLING_RATES_INTERVAL", 50*time.Second)

	conf.AddConfigPath(path)
	for _, file := range *files {
		conf.SetConfigFile(file)
	}
	// https://dev.to/techschoolguru/load-config-from-file-environment-variables-in-golang-with-viper-2j2d
	conf.AutomaticEnv()
	if err := conf.ReadInConfig(); err != nil {
		if err, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			logger.Info("Using default settings, config file not found", zap.Error(err))
		} else {
			// Config file was found but another error was produced
			logger.Info("Config file was found, but an error occurred", zap.Error(err))
		}
	}
	doOnce.Do(func() {
		err := viper.Unmarshal(&cfg)
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

func GetDBConnectionString(config Config) string {
	// Construct the connection string using the loaded configuration
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
