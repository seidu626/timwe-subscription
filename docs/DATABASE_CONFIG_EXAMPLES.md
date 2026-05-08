# Database Configuration Examples

This document provides examples of how to use the new unified database connection pool configuration in the `common/postgres` package.

## Overview

The database configuration has been completely refactored to provide a unified configuration system that:

1. **Integrates with main config** - Loads settings from the main application configuration
2. **Supports environment variables** - Runtime configuration without code changes
3. **Provides configuration structs** - Direct programmatic configuration
4. **Maintains backward compatibility** - Works with existing code
5. **Unifies all database settings** - Single source of truth for database configuration

## Configuration Sources

The database configuration can be loaded from multiple sources in order of priority:

1. **Direct configuration struct** (highest priority)
2. **Main application config file** (medium priority)
3. **Environment variables** (lowest priority)
4. **Default values** (fallback)

## Basic Usage

### 1. Using Main Application Config (Recommended)

```go
package main

import (
    "context"
    "log"
    
    "github.com/seidu626/subscription-manager/common/config"
    "github.com/seidu626/subscription-manager/common/postgres"
)

func main() {
    ctx := context.Background()
    
    // Load main application config
    mainConfig := config.InitConfig(logger, "config", []string{"config.yaml"})
    
    // Create database pool using main config
    pool, err := postgres.NewPGXPoolFromMainConfig(
        ctx, 
        mainConfig, 
        &postgres.PGXStdLogger{}, 
        postgres.LogLevelFromEnv(),
    )
    if err != nil {
        log.Fatal("Failed to create database pool:", err)
    }
    defer pool.Close()
    
    // Use the pool...
}
```

### 2. Using Default Configuration

```go
package main

import (
    "context"
    "log"
    
    "github.com/seidu626/subscription-manager/common/postgres"
)

func main() {
    ctx := context.Background()
    
    // Use default configuration (recommended for development)
    pool, err := postgres.NewPGXPoolWithDefaults(
        ctx, 
        "", // Empty string uses config defaults
        &postgres.PGXStdLogger{}, 
        postgres.LogLevelFromEnv(),
    )
    if err != nil {
        log.Fatal("Failed to create database pool:", err)
    }
    defer pool.Close()
    
    // Use the pool...
}
```

### 3. Using Environment Variables

```go
package main

import (
    "context"
    "log"
    
    "github.com/seidu626/subscription-manager/common/postgres"
)

func main() {
    ctx := context.Background()
    
    // Automatically use environment variables for configuration
    pool, err := postgres.NewPGXPoolFromEnv(
        ctx, 
        "", 
        &postgres.PGXStdLogger{}, 
        postgres.LogLevelFromEnv(),
    )
    if err != nil {
        log.Fatal("Failed to create database pool:", err)
    }
    defer pool.Close()
    
    // Use the pool...
}
```

### 4. Using Custom Configuration

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/seidu626/subscription-manager/common/postgres"
)

func main() {
    ctx := context.Background()
    
    // Create custom configuration
    config := &postgres.DatabaseConfig{
        // Connection settings
        Host:     "my-db-host",
        Port:     "5432",
        User:     "myuser",
        Password: "mypassword",
        DBName:   "mydatabase",
        SSLMode:  "require",
        
        // Connection pool settings
        MaxOpenConns:    100,
        MaxIdleConns:    20,
        ConnMaxLifetime: 2 * time.Hour,
        
        // Timeout settings
        QueryTimeout:       45 * time.Second,
        ConnectionTimeout:  15 * time.Second,
        TransactionTimeout: 45 * time.Second,
        PoolTimeout:        30 * time.Second,
        
        // Advanced pgx settings
        ConnectTimeout:                   15 * time.Second,
        StatementTimeout:                 45 * time.Second,
        IdleInTransactionSessionTimeout:  45 * time.Second,
        MaxConns:                         100,
        MinConns:                         10,
        MaxConnLifetime:                  2 * time.Hour,
        MaxConnIdleTime:                  45 * time.Minute,
        HealthCheckPeriod:                30 * time.Second,
    }
    
    pool, err := postgres.NewPGXPoolFromConfig(
        ctx, 
        config, 
        &postgres.PGXStdLogger{}, 
        postgres.LogLevelFromEnv(),
    )
    if err != nil {
        log.Fatal("Failed to create database pool:", err)
    }
    defer pool.Close()
    
    // Use the pool...
}
```

## Main Application Config Integration

### Configuration File Structure

```yaml
APPLICATION:
  DATABASE:
    POSTGRESQL:
      HOST: "localhost"
      PORT: "5432"
      USER: "postgres"
      PASSWORD: "password"
      DB_NAME: "subscription_db"
      SSL_MODE: "disable"
      
      # Connection pool settings
      MAX_OPEN_CONNS: 100
      MAX_IDLE_CONNS: 20
      CONN_MAX_LIFETIME: "2h"
      
      # Enhanced database timeout settings
      QUERY_TIMEOUT: "45s"
      CONNECTION_TIMEOUT: "15s"
      TRANSACTION_TIMEOUT: "45s"
      POOL_TIMEOUT: "30s"
```

### Loading from Main Config

```go
// The database configuration is automatically loaded from the main config
// when using NewPGXPoolFromMainConfig()
mainConfig := config.InitConfig(logger, "config", []string{"config.yaml"})

pool, err := postgres.NewPGXPoolFromMainConfig(ctx, mainConfig, logger, logLevel)
```

## Environment Variable Configuration

### Setting Environment Variables

#### Linux/macOS
```bash
# Basic connection settings
export PGHOST="my-db-host"
export PGPORT="5432"
export PGUSER="myuser"
export PGPASSWORD="mypassword"
export PGDATABASE="mydatabase"
export PGSSLMODE="require"

# Connection pool settings
export PGMAX_OPEN_CONNS="100"
export PGMAX_IDLE_CONNS="20"
export PGCONN_MAX_LIFETIME="7200"

# Timeout settings
export PGQUERY_TIMEOUT="45"
export PGCONNECTION_TIMEOUT="15"
export PGTRANSACTION_TIMEOUT="45"
export PGPOOL_TIMEOUT="30"

# Advanced pgx settings
export PGCONNECT_TIMEOUT="15"
export PGSTATEMENT_TIMEOUT="45"
export PGIDLE_IN_TRANSACTION_SESSION_TIMEOUT="45"
export PGMAX_CONNS="100"
export PGMIN_CONNS="10"
export PGMAX_CONN_LIFETIME="7200"
export PGMAX_CONN_IDLE_TIME="2700"
export PGHEALTH_CHECK_PERIOD="30"
```

#### Docker Compose
```yaml
version: '3.8'
services:
  app:
    environment:
      # Basic connection settings
      - PGHOST=my-db-host
      - PGPORT=5432
      - PGUSER=myuser
      - PGPASSWORD=mypassword
      - PGDATABASE=mydatabase
      - PGSSLMODE=require
      
      # Connection pool settings
      - PGMAX_OPEN_CONNS=100
      - PGMAX_IDLE_CONNS=20
      - PGCONN_MAX_LIFETIME=7200
      
      # Timeout settings
      - PGQUERY_TIMEOUT=45
      - PGCONNECTION_TIMEOUT=15
      - PGTRANSACTION_TIMEOUT=45
      - PGPOOL_TIMEOUT=30
      
      # Advanced pgx settings
      - PGCONNECT_TIMEOUT=15
      - PGSTATEMENT_TIMEOUT=45
      - PGIDLE_IN_TRANSACTION_SESSION_TIMEOUT=45
      - PGMAX_CONNS=100
      - PGMIN_CONNS=10
      - PGMAX_CONN_LIFETIME=7200
      - PGMAX_CONN_IDLE_TIME=2700
      - PGHEALTH_CHECK_PERIOD=30
```

## Configuration Options Reference

### DatabaseConfig Struct

| Field | Type | Default | Description | Config Tag |
|-------|------|---------|-------------|------------|
| `Host` | `string` | `"localhost"` | Database host | `HOST` |
| `Port` | `string` | `"5432"` | Database port | `PORT` |
| `User` | `string` | `"postgres"` | Database user | `USER` |
| `Password` | `string` | `""` | Database password | `PASSWORD` |
| `DBName` | `string` | `"postgres"` | Database name | `DB_NAME` |
| `SSLMode` | `string` | `"disable"` | SSL mode | `SSL_MODE` |
| `MaxOpenConns` | `int` | `50` | Maximum open connections | `MAX_OPEN_CONNS` |
| `MaxIdleConns` | `int` | `10` | Maximum idle connections | `MAX_IDLE_CONNS` |
| `ConnMaxLifetime` | `time.Duration` | `1h` | Connection max lifetime | `CONN_MAX_LIFETIME` |
| `QueryTimeout` | `time.Duration` | `30s` | Query timeout | `QUERY_TIMEOUT` |
| `ConnectionTimeout` | `time.Duration` | `10s` | Connection timeout | `CONNECTION_TIMEOUT` |
| `TransactionTimeout` | `time.Duration` | `30s` | Transaction timeout | `TRANSACTION_TIMEOUT` |
| `PoolTimeout` | `time.Duration` | `30s` | Pool timeout | `POOL_TIMEOUT` |
| `ConnectTimeout` | `time.Duration` | `10s` | pgx connect timeout | - |
| `StatementTimeout` | `time.Duration` | `30s` | pgx statement timeout | - |
| `IdleInTransactionSessionTimeout` | `time.Duration` | `30s` | pgx idle transaction timeout | - |
| `MaxConns` | `int32` | `50` | pgx max connections | - |
| `MinConns` | `int32` | `5` | pgx min connections | - |
| `MaxConnLifetime` | `time.Duration` | `1h` | pgx max connection lifetime | - |
| `MaxConnIdleTime` | `time.Duration` | `30m` | pgx max idle time | - |
| `HealthCheckPeriod` | `time.Duration` | `1m` | pgx health check period | - |

### Environment Variables

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `PGHOST` | `string` | `"localhost"` | Database host |
| `PGPORT` | `string` | `"5432"` | Database port |
| `PGUSER` | `string` | `"postgres"` | Database user |
| `PGPASSWORD` | `string` | `""` | Database password |
| `PGDATABASE` | `string` | `"postgres"` | Database name |
| `PGSSLMODE` | `string` | `"disable"` | SSL mode |
| `PGMAX_OPEN_CONNS` | `int` | `50` | Maximum open connections |
| `PGMAX_IDLE_CONNS` | `int` | `10` | Maximum idle connections |
| `PGCONN_MAX_LIFETIME` | `int` | `3600` | Connection max lifetime in seconds |
| `PGQUERY_TIMEOUT` | `int` | `30` | Query timeout in seconds |
| `PGCONNECTION_TIMEOUT` | `int` | `10` | Connection timeout in seconds |
| `PGTRANSACTION_TIMEOUT` | `int` | `30` | Transaction timeout in seconds |
| `PGPOOL_TIMEOUT` | `int` | `30` | Pool timeout in seconds |
| `PGCONNECT_TIMEOUT` | `int` | `10` | pgx connect timeout in seconds |
| `PGSTATEMENT_TIMEOUT` | `int` | `30` | pgx statement timeout in seconds |
| `PGIDLE_IN_TRANSACTION_SESSION_TIMEOUT` | `int` | `30` | pgx idle transaction timeout in seconds |
| `PGMAX_CONNS` | `int` | `50` | pgx max connections |
| `PGMIN_CONNS` | `int` | `5` | pgx min connections |
| `PGMAX_CONN_LIFETIME` | `int` | `3600` | pgx max connection lifetime in seconds |
| `PGMAX_CONN_IDLE_TIME` | `int` | `1800` | pgx max idle time in seconds |
| `PGHEALTH_CHECK_PERIOD` | `int` | `60` | pgx health check period in seconds |

## Migration Guide

### From Old Hardcoded Version

#### Before (Old Version)
```go
// Old hardcoded version
pool, err := postgres.NewPGXPool(ctx, connString, logger, logLevel)
```

#### After (New Unified Version)
```go
// Option 1: Use main config (recommended for production)
mainConfig := config.InitConfig(logger, "config", []string{"config.yaml"})
pool, err := postgres.NewPGXPoolFromMainConfig(ctx, mainConfig, logger, logLevel)

// Option 2: Use defaults (recommended for development)
pool, err := postgres.NewPGXPoolWithDefaults(ctx, connString, logger, logLevel)

// Option 3: Use environment variables
pool, err := postgres.NewPGXPoolFromEnv(ctx, connString, logger, logLevel)

// Option 4: Use custom configuration
config := &postgres.DatabaseConfig{ /* your settings */ }
pool, err := postgres.NewPGXPoolFromConfig(ctx, config, logger, logLevel)
```

### From Main Config Only

#### Before (Separate Configs)
```go
// Load main config
mainConfig := config.InitConfig(logger, "config", []string{"config.yaml"})

// Manually extract database settings
dbHost := mainConfig.Database.Postgresql.Host
dbPort := mainConfig.Database.Postgresql.Port
// ... etc

// Create connection string manually
connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
    dbHost, dbPort, dbUser, dbPassword, dbName, dbSSLMode)

// Create pool with hardcoded settings
pool, err := postgres.NewPGXPool(ctx, connStr, logger, logLevel)
```

#### After (Unified Config)
```go
// Load main config
mainConfig := config.InitConfig(logger, "config", []string{"config.yaml"})

// Create pool using unified config system
pool, err := postgres.NewPGXPoolFromMainConfig(ctx, mainConfig, logger, logLevel)
```

## Best Practices

### 1. Development Environment
- Use `NewPGXPoolWithDefaults()` for simplicity
- Default values are optimized for development
- Override specific settings with environment variables if needed

### 2. Production Environment
- Use `NewPGXPoolFromMainConfig()` for centralized configuration
- Store configuration in version-controlled config files
- Use environment variables for sensitive information (passwords, keys)

### 3. Testing Environment
- Use custom configuration for specific test scenarios
- Set aggressive timeouts for fast test execution
- Use separate database instances for testing

### 4. Configuration Management
- Store sensitive configuration in environment variables
- Use configuration management tools (Vault, AWS Secrets Manager, etc.)
- Document all configuration options
- Use consistent naming conventions

## Monitoring and Metrics

### Connection Pool Statistics
```go
stats := pool.Stat()
log.Printf("Total connections: %d", stats.TotalConns())
log.Printf("Idle connections: %d", stats.IdleConns())
log.Printf("In-use connections: %d", stats.AcquiredConns())
log.Printf("Wait count: %d", stats.WaitCount())
```

### Health Checks
```go
// Check if pool is healthy
if err := pool.Ping(ctx); err != nil {
    log.Printf("Database pool health check failed: %v", err)
} else {
    log.Println("Database pool is healthy")
}
```

### Configuration Validation
```go
// Validate configuration before creating pool
config := &postgres.DatabaseConfig{
    Host: "localhost",
    Port: "5432",
    // ... other settings
}

// Validate required fields
if config.Host == "" || config.Port == "" || config.User == "" || config.DBName == "" {
    return fmt.Errorf("missing required database configuration")
}

pool, err := postgres.NewPGXPoolFromConfig(ctx, config, logger, logLevel)
```

## Troubleshooting

### Common Issues

1. **Configuration Not Loading**
   - Check config file path and format
   - Verify environment variable names
   - Ensure config struct tags match config file keys

2. **Connection Timeouts**
   - Increase timeout values in config
   - Check network connectivity
   - Verify database server is running

3. **Connection Pool Exhaustion**
   - Increase `MaxConns` and `MaxOpenConns`
   - Check for connection leaks
   - Monitor connection usage patterns

4. **Configuration Conflicts**
   - Check priority order (direct > main config > env vars > defaults)
   - Verify no conflicting settings
   - Use consistent units (seconds vs duration strings)

### Debug Mode
```go
// Enable debug logging
logLevel, _ := postgres.LogLevelFromEnv()
if logLevel == postgres.LogLevelDebug {
    // Debug information will be logged
    log.Printf("Database config: %+v", config)
}
```

## Conclusion

The new unified database configuration system provides:

- **Centralization**: Single source of truth for all database settings
- **Flexibility**: Multiple configuration sources with priority ordering
- **Integration**: Seamless integration with existing config systems
- **Maintainability**: No more scattered hardcoded values
- **Scalability**: Easy to adjust settings for different environments
- **Best Practices**: Follows Go and database configuration best practices

Choose the configuration method that best fits your deployment strategy and operational requirements. The unified system ensures consistency across all database connections while maintaining flexibility for different use cases. 