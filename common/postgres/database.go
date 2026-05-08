package postgres

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
)

// DatabaseConfig holds configuration for PostgreSQL connection pool
type DatabaseConfig struct {
	// Connection settings
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

	// Advanced connection pool settings (pgx specific)
	ConnectTimeout                  time.Duration
	StatementTimeout                time.Duration
	IdleInTransactionSessionTimeout time.Duration
	MaxConns                        int32
	MinConns                        int32
	MaxConnLifetime                 time.Duration
	MaxConnIdleTime                 time.Duration
	HealthCheckPeriod               time.Duration
}

// DefaultDatabaseConfig returns a DatabaseConfig with sensible defaults
func DefaultDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		// Connection defaults
		Host:     "localhost",
		Port:     "5432",
		User:     "postgres",
		Password: "",
		DBName:   "postgres",
		SSLMode:  "disable",

		// Connection pool defaults
		MaxOpenConns:    50,
		MaxIdleConns:    10,
		ConnMaxLifetime: 1 * time.Hour,

		// Timeout defaults
		QueryTimeout:       30 * time.Second,
		ConnectionTimeout:  10 * time.Second,
		TransactionTimeout: 30 * time.Second,
		PoolTimeout:        30 * time.Second,

		// Advanced pgx defaults
		ConnectTimeout:                  10 * time.Second,
		StatementTimeout:                30 * time.Second,
		IdleInTransactionSessionTimeout: 30 * time.Second,
		MaxConns:                        50,
		MinConns:                        5,
		MaxConnLifetime:                 1 * time.Hour,
		MaxConnIdleTime:                 30 * time.Minute,
		HealthCheckPeriod:               1 * time.Minute,
	}
}

// DatabaseConfigFromEnv creates a DatabaseConfig from environment variables
func DatabaseConfigFromEnv() *DatabaseConfig {
	config := DefaultDatabaseConfig()

	// Basic connection settings
	if host := os.Getenv("PGHOST"); host != "" {
		config.Host = host
	}
	if port := os.Getenv("PGPORT"); port != "" {
		config.Port = port
	}
	if user := os.Getenv("PGUSER"); user != "" {
		config.User = user
	}
	if password := os.Getenv("PGPASSWORD"); password != "" {
		config.Password = password
	}
	if dbname := os.Getenv("PGDATABASE"); dbname != "" {
		config.DBName = dbname
	}
	if sslmode := os.Getenv("PGSSLMODE"); sslmode != "" {
		config.SSLMode = sslmode
	}

	// Connection pool settings
	if maxOpenConns := os.Getenv("PGMAX_OPEN_CONNS"); maxOpenConns != "" {
		if val, err := strconv.Atoi(maxOpenConns); err == nil {
			config.MaxOpenConns = val
		}
	}
	if maxIdleConns := os.Getenv("PGMAX_IDLE_CONNS"); maxIdleConns != "" {
		if val, err := strconv.Atoi(maxIdleConns); err == nil {
			config.MaxIdleConns = val
		}
	}
	if connMaxLifetime := os.Getenv("PGCONN_MAX_LIFETIME"); connMaxLifetime != "" {
		if duration, err := time.ParseDuration(connMaxLifetime + "s"); err == nil {
			config.ConnMaxLifetime = duration
		}
	}

	// Timeout settings
	if queryTimeout := os.Getenv("PGQUERY_TIMEOUT"); queryTimeout != "" {
		if duration, err := time.ParseDuration(queryTimeout + "s"); err == nil {
			config.QueryTimeout = duration
		}
	}
	if connectionTimeout := os.Getenv("PGCONNECTION_TIMEOUT"); connectionTimeout != "" {
		if duration, err := time.ParseDuration(connectionTimeout + "s"); err == nil {
			config.ConnectionTimeout = duration
		}
	}
	if transactionTimeout := os.Getenv("PGTRANSACTION_TIMEOUT"); transactionTimeout != "" {
		if duration, err := time.ParseDuration(transactionTimeout + "s"); err == nil {
			config.TransactionTimeout = duration
		}
	}
	if poolTimeout := os.Getenv("PGPOOL_TIMEOUT"); poolTimeout != "" {
		if duration, err := time.ParseDuration(poolTimeout + "s"); err == nil {
			config.PoolTimeout = duration
		}
	}

	// Advanced pgx settings
	if timeout := os.Getenv("PGCONNECT_TIMEOUT"); timeout != "" {
		if duration, err := time.ParseDuration(timeout + "s"); err == nil {
			config.ConnectTimeout = duration
		}
	}
	if timeout := os.Getenv("PGSTATEMENT_TIMEOUT"); timeout != "" {
		if duration, err := time.ParseDuration(timeout + "s"); err == nil {
			config.StatementTimeout = duration
		}
	}
	if timeout := os.Getenv("PGIDLE_IN_TRANSACTION_SESSION_TIMEOUT"); timeout != "" {
		if duration, err := time.ParseDuration(timeout + "s"); err == nil {
			config.IdleInTransactionSessionTimeout = duration
		}
	}
	if maxConns := os.Getenv("PGMAX_CONNS"); maxConns != "" {
		if val, err := strconv.ParseInt(maxConns, 10, 32); err == nil {
			config.MaxConns = int32(val)
		}
	}
	if minConns := os.Getenv("PGMIN_CONNS"); minConns != "" {
		if val, err := strconv.ParseInt(minConns, 10, 32); err == nil {
			config.MinConns = int32(val)
		}
	}
	if lifetime := os.Getenv("PGMAX_CONN_LIFETIME"); lifetime != "" {
		if duration, err := time.ParseDuration(lifetime + "s"); err == nil {
			config.MaxConnLifetime = duration
		}
	}
	if idleTime := os.Getenv("PGMAX_CONN_IDLE_TIME"); idleTime != "" {
		if duration, err := time.ParseDuration(idleTime + "s"); err == nil {
			config.MaxConnIdleTime = duration
		}
	}
	if healthCheck := os.Getenv("PGHEALTH_CHECK_PERIOD"); healthCheck != "" {
		if duration, err := time.ParseDuration(healthCheck + "s"); err == nil {
			config.HealthCheckPeriod = duration
		}
	}

	return config
}

// GetConnectionString returns the PostgreSQL connection string from the config
func (c *DatabaseConfig) GetConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

// NewPGXPool is a PostgreSQL connection pool for pgx with configurable settings.
//
// Usage:
// pgPool := database.NewPGXPool(context.Background(), "", &PGXStdLogger{}, tracelog.LogLevelInfo, nil)
// defer pgPool.Close() // Close any remaining connections before shutting down your application.
//
// Instead of passing a configuration explictly with a connString,
// you might use PG environment variables such as the following to configure the database:
// PGDATABASE, PGHOST, PGPORT, PGUSER, PGPASSWORD, PGCONNECT_TIMEOUT, etc.
// Reference: https://www.postgresql.org/docs/current/libpq-envars.html
func NewPGXPool(ctx context.Context, connString string, logger tracelog.Logger, logLevel tracelog.LogLevel, config *DatabaseConfig) (*pgxpool.Pool, error) {
	// Use provided config or defaults
	if config == nil {
		config = DefaultDatabaseConfig()
	}

	// If connString is empty, build it from config
	if connString == "" {
		connString = config.GetConnectionString()
	}

	conf, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, err
	}

	conf.ConnConfig.Tracer = &tracelog.TraceLog{
		Logger:   logger,
		LogLevel: logLevel,
	}

	// Set connection timeouts to prevent hanging connections
	conf.ConnConfig.Config.ConnectTimeout = config.ConnectTimeout
	conf.ConnConfig.Config.RuntimeParams["statement_timeout"] = config.StatementTimeout.String()
	conf.ConnConfig.Config.RuntimeParams["idle_in_transaction_session_timeout"] = config.IdleInTransactionSessionTimeout.String()

	// Set connection pool limits to prevent resource exhaustion
	conf.MaxConns = config.MaxConns
	conf.MinConns = config.MinConns
	conf.MaxConnLifetime = config.MaxConnLifetime
	conf.MaxConnIdleTime = config.MaxConnIdleTime

	// Health check configuration
	conf.HealthCheckPeriod = config.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, conf)
	if err != nil {
		return nil, fmt.Errorf("pgx connection error: %w", err)
	}
	return pool, nil
}

// NewPGXPoolWithDefaults is a convenience function that uses default configuration
func NewPGXPoolWithDefaults(ctx context.Context, connString string, logger tracelog.Logger, logLevel tracelog.LogLevel) (*pgxpool.Pool, error) {
	return NewPGXPool(ctx, connString, logger, logLevel, nil)
}

// NewPGXPoolFromEnv creates a new PostgreSQL connection pool using environment variables for configuration
func NewPGXPoolFromEnv(ctx context.Context, connString string, logger tracelog.Logger, logLevel tracelog.LogLevel) (*pgxpool.Pool, error) {
	config := DatabaseConfigFromEnv()
	return NewPGXPool(ctx, connString, logger, logLevel, config)
}

// NewPGXPoolFromConfig creates a new PostgreSQL connection pool using a configuration struct
func NewPGXPoolFromConfig(ctx context.Context, config *DatabaseConfig, logger tracelog.Logger, logLevel tracelog.LogLevel) (*pgxpool.Pool, error) {
	return NewPGXPool(ctx, "", logger, logLevel, config)
}

// LoadDatabaseConfigFromMainConfig loads database configuration from the main application config
// This function integrates with the existing config system
func LoadDatabaseConfigFromMainConfig(mainConfig interface{}) (*DatabaseConfig, error) {
	config := DefaultDatabaseConfig()

	// Use reflection to extract database configuration from main config
	// This allows integration with various config structures
	if mainConfig != nil {
		// Try to extract database config using common patterns
		if extractor, ok := mainConfig.(interface{ GetDatabaseConfig() *DatabaseConfig }); ok {
			return extractor.GetDatabaseConfig(), nil
		}

		// Try to extract using mapstructure tags
		if extractor, ok := mainConfig.(interface{ GetDatabaseConfig() map[string]interface{} }); ok {
			dbConfigMap := extractor.GetDatabaseConfig()
			// Convert map to DatabaseConfig struct
			// This is a simplified approach - in practice you might want to use mapstructure
			if host, ok := dbConfigMap["HOST"].(string); ok {
				config.Host = host
			}
			if port, ok := dbConfigMap["PORT"].(string); ok {
				config.Port = port
			}
			if user, ok := dbConfigMap["USER"].(string); ok {
				config.User = user
			}
			if password, ok := dbConfigMap["PASSWORD"].(string); ok {
				config.Password = password
			}
			if dbname, ok := dbConfigMap["DB_NAME"].(string); ok {
				config.DBName = dbname
			}
			if sslmode, ok := dbConfigMap["SSL_MODE"].(string); ok {
				config.SSLMode = sslmode
			}
			if maxOpenConns, ok := dbConfigMap["MAX_OPEN_CONNS"].(int); ok {
				config.MaxOpenConns = maxOpenConns
			}
			if maxIdleConns, ok := dbConfigMap["MAX_IDLE_CONNS"].(int); ok {
				config.MaxIdleConns = maxIdleConns
			}
			if connMaxLifetime, ok := dbConfigMap["CONN_MAX_LIFETIME"].(time.Duration); ok {
				config.ConnMaxLifetime = connMaxLifetime
			}
			if queryTimeout, ok := dbConfigMap["QUERY_TIMEOUT"].(time.Duration); ok {
				config.QueryTimeout = queryTimeout
			}
			if connectionTimeout, ok := dbConfigMap["CONNECTION_TIMEOUT"].(time.Duration); ok {
				config.ConnectionTimeout = connectionTimeout
			}
			if transactionTimeout, ok := dbConfigMap["TRANSACTION_TIMEOUT"].(time.Duration); ok {
				config.TransactionTimeout = transactionTimeout
			}
			if poolTimeout, ok := dbConfigMap["POOL_TIMEOUT"].(time.Duration); ok {
				config.PoolTimeout = poolTimeout
			}
		}
	}

	return config, nil
}

// NewPGXPoolFromMainConfig creates a new PostgreSQL connection pool using the main application config
func NewPGXPoolFromMainConfig(ctx context.Context, mainConfig interface{}, logger tracelog.Logger, logLevel tracelog.LogLevel) (*pgxpool.Pool, error) {
	dbConfig, err := LoadDatabaseConfigFromMainConfig(mainConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load database config from main config: %w", err)
	}
	return NewPGXPoolFromConfig(ctx, dbConfig, logger, logLevel)
}

// LogLevelFromEnv returns the tracelog.LogLevel from the environment variable PGX_LOG_LEVEL.
// By default, this is info (tracelog.LogLevelInfo), which is good for development.
// For deployments, something like tracelog.LogLevelWarn is better choice.
func LogLevelFromEnv() (tracelog.LogLevel, error) {
	if level := os.Getenv("PGX_LOG_LEVEL"); level != "" {
		l, err := tracelog.LogLevelFromString(level)
		if err != nil {
			return tracelog.LogLevelDebug, fmt.Errorf("pgx configuration: %w", err)
		}
		return l, nil
	}
	return tracelog.LogLevelInfo, nil
}

// PGXStdLogger prints pgx logs to the standard logger.
// os.Stderr by default.
type PGXStdLogger struct{}

func (l *PGXStdLogger) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
	args := make([]any, 0, len(data)+2) // making space for arguments + level + msg
	args = append(args, level, msg)
	for k, v := range data {
		args = append(args, fmt.Sprintf("%s=%v", k, v))
	}
	log.Println(args...)
}

// PgErrors returns a multi-line error printing more information from *pgconn.PgError to make debugging faster.
func PgErrors(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	return fmt.Errorf(`%w
Code: %v
Detail: %v
Hint: %v
Position: %v
InternalPosition: %v
InternalQuery: %v
Where: %v
SchemaName: %v
TableName: %v
ColumnName: %v
DataTypeName: %v
ConstraintName: %v
File: %v:%v
Routine: %v`,
		err,
		pgErr.Code,
		pgErr.Detail,
		pgErr.Hint,
		pgErr.Position,
		pgErr.InternalPosition,
		pgErr.InternalQuery,
		pgErr.Where,
		pgErr.SchemaName,
		pgErr.TableName,
		pgErr.ColumnName,
		pgErr.DataTypeName,
		pgErr.ConstraintName,
		pgErr.File, pgErr.Line,
		pgErr.Routine)
}
