# Rolling Log Implementation

## Overview

The subscription-external service now supports rolling log files with configurable size limits, rotation settings, datetime-based filenames, and intelligent compression thresholds. This implementation uses the `lumberjack` library to provide automatic log file rotation, compression, and cleanup.

## Features

- **Automatic Log Rotation**: Logs are automatically rotated when they reach a specified size
- **Configurable Limits**: Set maximum file size, age, and number of backup files
- **Intelligent Compression**: Compress files only when the count exceeds a specified threshold
- **Datetime Filenames**: Log files include timestamps in their names for better organization
- **Dual Output**: Logs are written to both console (stdout) and file simultaneously
- **JSON Format**: Structured logging in JSON format for easy parsing and analysis

## Configuration

The rolling log functionality is configured in `config.yaml`:

```yaml
APPLICATION:
  LOG:
    PATH: /home/xper626/logs/subscription-external-app.log
    ROLLING:
      ENABLED: true
      MAX_SIZE: 100    # MB
      MAX_AGE: 30      # days
      MAX_BACKUPS: 10  # number of backup files
      COMPRESS: true   # enable compression
      COMPRESS_THRESHOLD: 10  # only compress when file count exceeds this number
      LOCAL_TIME: true # use local time for rotation
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `ENABLED` | bool | false | Enable/disable rolling log functionality |
| `MAX_SIZE` | int | 100 | Maximum size of log file in MB before rotation |
| `MAX_AGE` | int | 30 | Maximum age of log files in days before deletion |
| `MAX_BACKUPS` | int | 10 | Maximum number of backup files to keep |
| `COMPRESS` | bool | true | Enable compression of rotated files |
| `COMPRESS_THRESHOLD` | int | 10 | Only compress when file count exceeds this number |
| `LOCAL_TIME` | bool | true | Use local time instead of UTC for rotation |

## Implementation Details

### Dependencies

- `gopkg.in/natefinch/lumberjack.v2` - For log file rotation and management
- `go.uber.org/zap` - For structured logging

### Key Functions

#### `NewRollingFileLogger(logFilePath string, rollingConfig RollingLogConfig) (*zap.Logger, error)`

Creates a new zap logger with rolling file output. This function:

1. Creates a datetime-based filename using `createDatetimeFilename()`
2. Creates a lumberjack logger with the specified configuration
3. Sets up a multi-writer that writes to both console and file
4. Configures JSON encoding with proper field names
5. Falls back to console-only logging if rolling is disabled

#### `RollingLogConfig` Struct

```go
type RollingLogConfig struct {
    Enabled           bool `yaml:"enabled"`
    MaxSize           int  `yaml:"max_size"`           // MB
    MaxAge            int  `yaml:"max_age"`            // days
    MaxBackups        int  `yaml:"max_backups"`        // number of backup files
    Compress          bool `yaml:"compress"`           // enable compression
    CompressThreshold int  `yaml:"compress_threshold"` // only compress when file count exceeds this number
    LocalTime         bool `yaml:"local_time"`         // use local time for rotation
}
```

#### `createDatetimeFilename(basePath string) string`

Creates a filename with datetime suffix in the format `YYYY-MM-DD_HH-MM-SS`:

```go
// Example: /var/log/app.log -> /var/log/app_2025-08-20_15-00-04.log
func createDatetimeFilename(basePath string) string {
    now := time.Now()
    ext := ".log"
    
    dir := filepath.Dir(basePath)
    baseName := filepath.Base(basePath)
    
    // Remove .log extension if present
    if strings.HasSuffix(baseName, ".log") {
        baseName = strings.TrimSuffix(baseName, ".log")
    }
    
    // Create datetime suffix
    datetimeSuffix := now.Format("2006-01-02_15-04-05")
    
    // Combine parts
    filename := fmt.Sprintf("%s_%s%s", baseName, datetimeSuffix, ext)
    return filepath.Join(dir, filename)
}
```

#### `shouldCompress(config RollingLogConfig) bool`

Determines whether to compress files based on the threshold:

```go
func shouldCompress(config RollingLogConfig) bool {
    if !config.Compress {
        return false
    }
    
    // If no threshold is set, always compress
    if config.CompressThreshold <= 0 {
        return true
    }
    
    // Only compress when the number of files exceeds the threshold
    return config.MaxBackups > config.CompressThreshold
}
```

## Usage

### In main.go

```go
// Initialize rolling file logger if enabled
var logger *zap.Logger
if cfg.Application.Log.Rolling.Enabled {
    rollingConfig := logging.RollingLogConfig{
        Enabled:           cfg.Application.Log.Rolling.Enabled,
        MaxSize:           cfg.Application.Log.Rolling.MaxSize,
        MaxAge:            cfg.Application.Log.Rolling.MaxAge,
        MaxBackups:        cfg.Application.Log.Rolling.MaxBackups,
        Compress:          cfg.Application.Log.Rolling.Compress,
        CompressThreshold: cfg.Application.Log.Rolling.CompressThreshold,
        LocalTime:         cfg.Application.Log.Rolling.LocalTime,
    }
    
    logger, err = logging.NewRollingFileLogger(cfg.Application.Log.Path, rollingConfig)
    if err != nil {
        log.Fatalf("could not initialize rolling file logger: %v", err)
    }
} else {
    // Fallback to basic logger
    logger = basicLogger
}
```

### Logging Messages

```go
logger.Info("Application started")
logger.Error("An error occurred", zap.Error(err))
logger.Warn("Warning message", zap.String("component", "database"))
```

## Log File Rotation

When a log file reaches the specified `MAX_SIZE`:

1. The current log file is renamed with a datetime suffix
2. A new log file is created for new log entries
3. If `COMPRESS` is enabled and file count exceeds `COMPRESS_THRESHOLD`, the rotated file is compressed
4. If the number of backup files exceeds `MAX_BACKUPS`, the oldest files are deleted
5. Files older than `MAX_AGE` days are automatically deleted

### Example File Structure

```
/home/xper626/logs/
├── subscription-external-app_2025-08-20_15-00-04.log  # Current log file
├── subscription-external-app_2025-08-20_14-30-15.log  # Previous log file
├── subscription-external-app_2025-08-20_14-00-22.log  # Previous log file
└── subscription-external-app_2025-08-20_13-30-45.log  # Previous log file
```

### Compression Strategy

The compression strategy is intelligent and configurable:

- **Always Compress**: If `COMPRESS_THRESHOLD` is 0 or negative, all files are compressed
- **Threshold-Based**: If `COMPRESS_THRESHOLD` > 0, only compress when `MAX_BACKUPS` > `COMPRESS_THRESHOLD`
- **Example**: With `MAX_BACKUPS: 15` and `COMPRESS_THRESHOLD: 10`, compression only occurs when there are more than 10 files

## Benefits

1. **Disk Space Management**: Automatic cleanup prevents log files from consuming excessive disk space
2. **Performance**: Smaller log files are easier to read and process
3. **Maintenance**: No manual intervention required for log rotation
4. **Structured Logging**: JSON format enables easy parsing and analysis
5. **Dual Output**: Console output for development, file output for production
6. **Intelligent Compression**: Saves disk space while maintaining performance for recent logs
7. **Organized Filenames**: Datetime suffixes make it easy to identify and manage log files

## Troubleshooting

### Common Issues

1. **Permission Denied**: Ensure the application has write permissions to the log directory
2. **Disk Space**: Monitor available disk space, especially with compression disabled
3. **File Rotation**: Check that the log file path is writable and the directory exists
4. **Compression**: Verify that the compression threshold is set appropriately for your use case

### Debugging

To debug logging issues:

1. Check console output for immediate feedback
2. Verify log file permissions and directory existence
3. Test with a simple log message to ensure basic functionality
4. Check the lumberjack configuration parameters
5. Verify datetime filename creation is working correctly
6. Monitor compression behavior based on file count

## Future Enhancements

Potential improvements for the rolling log system:

1. **Log Level Filtering**: Different log levels to different files
2. **Custom Rotation Triggers**: Time-based rotation in addition to size-based
3. **Log Aggregation**: Integration with centralized logging systems
4. **Metrics**: Log rotation statistics and monitoring
5. **Custom Formatters**: Support for different log formats beyond JSON
6. **Advanced Compression**: Different compression algorithms or levels
7. **File Naming Patterns**: Customizable datetime formats or naming conventions 