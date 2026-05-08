package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// RollingLogConfig holds configuration for rolling log files
type RollingLogConfig struct {
	Enabled           bool `yaml:"enabled"`
	MaxSize           int  `yaml:"max_size"`           // MB
	MaxAge            int  `yaml:"max_age"`            // days
	MaxBackups        int  `yaml:"max_backups"`        // number of backup files
	Compress          bool `yaml:"compress"`           // compress rotated files
	CompressThreshold int  `yaml:"compress_threshold"` // only compress when file count exceeds this number
	LocalTime         bool `yaml:"local_time"`         // use local time for rotation
}

// NewRollingFileLogger creates a new zap logger with rolling file output
func NewRollingFileLogger(logFilePath string, rollingConfig RollingLogConfig) (*zap.Logger, error) {
	// Add rolling file output if enabled
	if rollingConfig.Enabled && logFilePath != "" {
		// Create datetime-based filename
		datetimeFilename := createDatetimeFilename(logFilePath)

		rollingWriter := &lumberjack.Logger{
			Filename:   datetimeFilename,
			MaxSize:    rollingConfig.MaxSize,
			MaxAge:     rollingConfig.MaxAge,
			MaxBackups: rollingConfig.MaxBackups,
			Compress:   shouldCompress(rollingConfig),
			LocalTime:  rollingConfig.LocalTime,
		}

		// Create a multi-writer that writes to both console and file
		multiWriter := zapcore.NewMultiWriteSyncer(
			zapcore.AddSync(os.Stdout),
			zapcore.AddSync(rollingWriter),
		)

		encodeConfig := zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}

		zapConfig := zap.Config{
			Level:            zap.NewAtomicLevelAt(zap.InfoLevel),
			Development:      false,
			Encoding:         "json",
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
			EncoderConfig:    encodeConfig,
		}

		// Build the logger with rolling file support
		logger, err := zapConfig.Build(
			zap.WrapCore(func(core zapcore.Core) zapcore.Core {
				return zapcore.NewCore(
					zapcore.NewJSONEncoder(encodeConfig),
					multiWriter,
					zapcore.InfoLevel,
				)
			}),
		)
		if err != nil {
			return nil, err
		}

		return logger, nil
	}

	// Fallback to console-only logging if rolling is disabled
	return NewZapLogger("")
}

// createDatetimeFilename creates a filename with datetime suffix
func createDatetimeFilename(basePath string) string {
	now := time.Now()
	ext := ".log"

	// Extract directory and base filename
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

// shouldCompress determines whether to compress files based on the threshold
func shouldCompress(config RollingLogConfig) bool {
	if !config.Compress {
		return false
	}

	// If no threshold is set, always compress
	if config.CompressThreshold <= 0 {
		return true
	}

	// Only compress when the number of files exceeds the threshold
	// This is a simple heuristic - in practice, lumberjack handles the actual file count
	return config.MaxBackups > config.CompressThreshold
}

// NewZapLogger InitLogger initializes a zap logger with optional file output
func NewZapLogger(logFilePath string) (*zap.Logger, error) {
	outputPaths := []string{"stdout"}
	errorOutputPaths := []string{"stderr"}

	// Add file output path only if logFilePath is provided
	if logFilePath != "" {
		outputPaths = append(outputPaths, logFilePath)
		errorOutputPaths = append(errorOutputPaths, logFilePath)
	}

	encodeConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	zapConfig := zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.InfoLevel),
		Development: false,
		Encoding:    "json",
		//Sampling:         nil,
		OutputPaths:      outputPaths,
		ErrorOutputPaths: errorOutputPaths,
		EncoderConfig:    encodeConfig,
	}

	// Build the logger with the specified configuration
	logger, err := zapConfig.Build()
	if err != nil {
		return nil, err
	}

	return logger, nil
}

func NewZapFileConsoleLogger() *zap.Logger {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	fileEncoder := zapcore.NewJSONEncoder(cfg)
	consoleEncoder := zapcore.NewConsoleEncoder(cfg)
	logFile, _ := os.OpenFile("app_log.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	writer := zapcore.AddSync(logFile)
	defaultLogLevel := zapcore.DebugLevel
	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, writer, defaultLogLevel),
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), defaultLogLevel),
	)
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
}

// SetOutput replaces existing Core with new, that writes to passed WriteSyncer.
func SetOutput(ws zapcore.WriteSyncer, conf zap.Config) zap.Option {
	var enc zapcore.Encoder
	switch conf.Encoding {
	case "json":
		enc = zapcore.NewJSONEncoder(conf.EncoderConfig)
	case "console":
		enc = zapcore.NewConsoleEncoder(conf.EncoderConfig)
	default:
		panic("unknown encoding")
	}

	return zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewCore(enc, ws, conf.Level)
	})
}

func getWriteSyncer(logfileName string) zapcore.WriteSyncer {
	swSugar := zapcore.NewMultiWriteSyncer(
		zapcore.AddSync(os.Stdout),
		getWriteSyncer(logfileName),
	)
	return swSugar
}
