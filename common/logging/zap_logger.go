package logging

import (
	"os"

	"github.com/seidu626/subscription-manager/common/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewZapLogger InitLogger initializes a zap logger with optional file output
func NewZapLogger(logFilePath string, cfg *config.Config) (*zap.Logger, error) {
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
		Development: cfg.Application.Environment == config.DEVELOPMENT,
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
