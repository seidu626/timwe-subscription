package utils

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"strconv"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// Global panic handler instance
var globalPanicHandler *PanicHandler

// InitPanicHandler initializes the global panic handler
func InitPanicHandler(logger *zap.Logger) error {
	// Load configuration
	config, err := LoadPanicHandlerConfig("config/panic_handler.yaml", GetEnvironment())
	if err != nil {
		logger.Warn("Failed to load panic handler config, using defaults", zap.Error(err))
		config = DefaultPanicHandlerConfig()
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		logger.Error("Invalid panic handler config", zap.Error(err))
		return err
	}

	// Create global panic handler
	globalPanicHandler = NewPanicHandler(logger, config)

	logger.Info("Global panic handler initialized",
		zap.Bool("enable_recovery", config.EnableRecovery),
		zap.Bool("log_stack_traces", config.LogStackTraces),
		zap.Bool("exit_on_fatal", config.ExitOnFatal),
		zap.Duration("recovery_timeout", config.RecoveryTimeout),
	)

	return nil
}

// GetGlobalPanicHandler returns the global panic handler instance
func GetGlobalPanicHandler() *PanicHandler {
	return globalPanicHandler
}

// AlertChannel represents a notification channel for alerts
type AlertChannel interface {
	SendAlert(alert *PanicAlert) error
	GetName() string
}

// PanicAlert represents a panic alert that should be sent
type PanicAlert struct {
	Severity    string                 `json:"severity"`
	Message     string                 `json:"message"`
	Timestamp   time.Time              `json:"timestamp"`
	PanicValue  interface{}            `json:"panic_value"`
	PanicType   string                 `json:"panic_type"`
	PanicDepth  int32                  `json:"panic_depth"`
	TotalPanics int64                  `json:"total_panics"`
	MemoryUsage uint64                 `json:"memory_usage_mb"`
	Context     map[string]interface{} `json:"context,omitempty"`
}

// PanicEvent represents a single panic event for history tracking
type PanicEvent struct {
	Timestamp   time.Time
	Value       interface{}
	Type        string
	Depth       int32
	MemoryUsage uint64
}

// PanicHandler handles panics and fatal errors with comprehensive logging and recovery
type PanicHandler struct {
	logger *zap.Logger
	config *PanicHandlerConfig
	mu     sync.RWMutex

	// Self-protection mechanisms
	panicDepth    int32        // Track panic nesting depth
	maxPanicDepth int32        // Maximum allowed panic depth
	lastPanicTime time.Time    // Last panic time for rate limiting
	panicCount    int64        // Total panic count for this instance
	recoveryState atomic.Value // Current recovery state

	// Alerting and monitoring
	alertChannels []AlertChannel // Alert channels for notifications
	panicHistory  []PanicEvent   // Recent panic history for analysis
	alertMutex    sync.RWMutex   // Protect alert operations

	// Performance optimization
	panicQueue   chan *PanicEvent // Async panic processing queue
	workerPool   chan struct{}    // Worker pool for panic processing
	metricsCache *MetricsCache    // Cached metrics for performance
	stopChan     chan struct{}    // Stop signal for background workers
}

// MetricsCache caches frequently accessed metrics
type MetricsCache struct {
	lastUpdate    time.Time
	memoryStats   runtime.MemStats
	panicCount    int64
	recoveryState *RecoveryState
	mu            sync.RWMutex
}

// RecoveryState represents the current state of panic recovery
type RecoveryState struct {
	IsRecovering   bool
	RecoveryStart  time.Time
	PanicDepth     int32
	LastPanicValue interface{}
}

// PanicHandlerConfig contains configuration for the panic handler
type PanicHandlerConfig struct {
	EnableRecovery   bool          `yaml:"enable_recovery" env:"PANIC_ENABLE_RECOVERY"`
	LogStackTraces   bool          `yaml:"log_stack_traces" env:"PANIC_LOG_STACK_TRACES"`
	LogGoroutineInfo bool          `yaml:"log_goroutine_info" env:"PANIC_LOG_GOROUTINE_INFO"`
	MaxStackDepth    int           `yaml:"max_stack_depth" env:"PANIC_MAX_STACK_DEPTH"`
	RecoveryTimeout  time.Duration `yaml:"recovery_timeout" env:"PANIC_RECOVERY_TIMEOUT"`
	ExitOnFatal      bool          `yaml:"exit_on_fatal" env:"PANIC_EXIT_ON_FATAL"`
	ExitCode         int           `yaml:"exit_code" env:"PANIC_EXIT_CODE"`

	// Self-protection settings
	MaxPanicDepth int32 `yaml:"max_panic_depth" env:"PANIC_MAX_PANIC_DEPTH"`

	// Memory management settings
	MaxMemoryUsage         uint64        `yaml:"max_memory_usage_mb" env:"PANIC_MAX_MEMORY_USAGE_MB"`                 // in MB
	MemoryCleanupThreshold uint64        `yaml:"memory_cleanup_threshold_mb" env:"PANIC_MEMORY_CLEANUP_THRESHOLD_MB"` // in MB
	EnableMemoryMonitoring bool          `yaml:"enable_memory_monitoring" env:"PANIC_ENABLE_MEMORY_MONITORING"`
	MemoryCheckInterval    time.Duration `yaml:"memory_check_interval" env:"PANIC_MEMORY_CHECK_INTERVAL"`

	// Rate limiting settings
	MaxPanicsPerSecond int `yaml:"max_panics_per_second" env:"PANIC_MAX_PANICS_PER_SECOND"`
	PanicBurstLimit    int `yaml:"panic_burst_limit" env:"PANIC_BURST_LIMIT"`
}

// DefaultPanicHandlerConfig returns a default configuration
func DefaultPanicHandlerConfig() *PanicHandlerConfig {
	return &PanicHandlerConfig{
		EnableRecovery:   true,
		LogStackTraces:   true,
		LogGoroutineInfo: true,
		MaxStackDepth:    64,
		RecoveryTimeout:  30 * time.Second,
		ExitOnFatal:      true,
		ExitCode:         1,

		// Self-protection defaults
		MaxPanicDepth: 3,

		// Memory management defaults
		MaxMemoryUsage:         1024, // 1GB
		MemoryCleanupThreshold: 512,  // 512MB
		EnableMemoryMonitoring: true,
		MemoryCheckInterval:    5 * time.Second,

		// Rate limiting defaults
		MaxPanicsPerSecond: 10,
		PanicBurstLimit:    20,
	}
}

// NewPanicHandler creates a new panic handler with self-protection and performance optimization
func NewPanicHandler(logger *zap.Logger, config *PanicHandlerConfig) *PanicHandler {
	if config == nil {
		config = DefaultPanicHandlerConfig()
	}

	ph := &PanicHandler{
		logger:        logger,
		config:        config,
		maxPanicDepth: config.MaxPanicDepth, // Use config value

		// Performance optimization
		panicQueue:   make(chan *PanicEvent, 1000), // Buffer for 1000 panics
		workerPool:   make(chan struct{}, 10),      // 10 concurrent workers
		metricsCache: &MetricsCache{},
		stopChan:     make(chan struct{}),
	}

	// Initialize recovery state
	ph.recoveryState.Store(&RecoveryState{
		IsRecovering:   false,
		RecoveryStart:  time.Time{},
		PanicDepth:     0,
		LastPanicValue: nil,
	})

	// Start background workers
	ph.startBackgroundWorkers()

	// Start memory monitoring
	ph.startMemoryMonitoring()

	return ph
}

// RecoverPanic recovers from a panic and logs it comprehensively
func (ph *PanicHandler) RecoverPanic() {
	if r := recover(); r != nil {
		ph.HandlePanic(r, nil)
	}
}

// RecoverPanicWithContext recovers from a panic with additional context
func (ph *PanicHandler) RecoverPanicWithContext(ctx context.Context) {
	if r := recover(); r != nil {
		ph.HandlePanic(r, ctx)
	}
}

// RecoverPanicWithCallback recovers from a panic and executes a callback
func (ph *PanicHandler) RecoverPanicWithCallback(callback func(interface{})) {
	if r := recover(); r != nil {
		ph.HandlePanic(r, nil)
		if callback != nil {
			callback(r)
		}
	}
}

// SafeGo runs a function in a goroutine with panic recovery
func (ph *PanicHandler) SafeGo(f func()) {
	go func() {
		defer ph.RecoverPanic()
		f()
	}()
}

// SafeGoWithContext runs a function in a goroutine with panic recovery and context
func (ph *PanicHandler) SafeGoWithContext(ctx context.Context, f func(context.Context)) {
	go func() {
		defer ph.RecoverPanicWithContext(ctx)
		f(ctx)
	}()
}

// SafeExecute executes a function with panic recovery
func (ph *PanicHandler) SafeExecute(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			ph.HandlePanic(r, nil)
			err = fmt.Errorf("panic recovered: %v", r)
		}
	}()
	f()
	return nil
}

// SafeExecuteWithContext executes a function with panic recovery and context
func (ph *PanicHandler) SafeExecuteWithContext(ctx context.Context, f func(context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			ph.HandlePanic(r, ctx)
			err = fmt.Errorf("panic recovered: %v", r)
		}
	}()
	return f(ctx)
}

// HandlePanic handles a recovered panic comprehensively with self-protection
func (ph *PanicHandler) HandlePanic(r interface{}, ctx context.Context) {
	// Self-protection: Check panic depth
	currentDepth := atomic.AddInt32(&ph.panicDepth, 1)
	defer atomic.AddInt32(&ph.panicDepth, -1)

	if currentDepth > ph.maxPanicDepth {
		// Emergency fallback: just log and exit to prevent infinite loops
		ph.emergencyFallback(r, currentDepth)
		return
	}

	// Self-protection: Rate limiting
	if !ph.checkRateLimit() {
		ph.logger.Warn("PANIC RATE LIMIT EXCEEDED - skipping panic handling",
			zap.Any("panic_value", r),
			zap.Int32("panic_depth", currentDepth),
			zap.Time("last_panic_time", ph.lastPanicTime),
		)
		return
	}

	// Self-protection: Check if already recovering
	if !ph.beginRecovery(r, currentDepth) {
		ph.logger.Warn("PANIC RECOVERY ALREADY IN PROGRESS - skipping",
			zap.Any("panic_value", r),
			zap.Int32("panic_depth", currentDepth),
		)
		return
	}
	defer ph.endRecovery()

	// Update panic count
	atomic.AddInt64(&ph.panicCount, 1)

	// Get stack trace with depth limit
	stack := ph.getStackTrace()

	// Get caller information
	caller := ph.getCallerInfo()

	// Get goroutine information
	var goroutineInfo string
	if ph.config.LogGoroutineInfo {
		goroutineInfo = fmt.Sprintf("Goroutines: %d", runtime.NumGoroutine())
	}

	// Log the panic with comprehensive information
	ph.logger.Error("PANIC RECOVERED",
		zap.Any("panic_value", r),
		zap.String("panic_type", fmt.Sprintf("%T", r)),
		zap.String("caller", caller),
		zap.String("goroutine_info", goroutineInfo),
		zap.String("timestamp", time.Now().Format(time.RFC3339)),
		zap.Int32("panic_depth", currentDepth),
		zap.Int64("total_panic_count", atomic.LoadInt64(&ph.panicCount)),
	)

	// Log stack trace if enabled
	if ph.config.LogStackTraces {
		ph.logger.Error("PANIC STACK TRACE",
			zap.String("stack_trace", string(stack)),
		)
	}

	// Log context information if available
	if ctx != nil {
		if deadline, ok := ctx.Deadline(); ok {
			ph.logger.Error("PANIC CONTEXT INFO",
				zap.Time("deadline", deadline),
				zap.Bool("deadline_exceeded", time.Now().After(deadline)),
			)
		}
	}

	// Log system information
	ph.logSystemInfo()

	// Check alert conditions and send alerts if needed
	ph.checkAlertConditions(r, currentDepth)

	// Execute recovery logic if enabled
	if ph.config.EnableRecovery {
		ph.executeRecovery(r, stack)
	}

	// Exit if configured to do so
	if ph.config.ExitOnFatal {
		ph.logger.Fatal("Application terminating due to panic",
			zap.Any("panic_value", r),
			zap.Int("exit_code", ph.config.ExitCode),
			zap.Int32("panic_depth", currentDepth),
		)
		os.Exit(ph.config.ExitCode)
	}
}

// emergencyFallback handles critical panic scenarios to prevent infinite loops
func (ph *PanicHandler) emergencyFallback(r interface{}, depth int32) {
	// Use a simple fallback logger to avoid complex operations
	fallbackLogger := zap.NewNop()
	if ph.logger != nil {
		fallbackLogger = ph.logger
	}

	fallbackLogger.Error("EMERGENCY PANIC FALLBACK - preventing infinite loop",
		zap.Any("panic_value", r),
		zap.Int32("panic_depth", depth),
		zap.String("timestamp", time.Now().Format(time.RFC3339)),
	)

	// Force exit to prevent system collapse
	os.Exit(2)
}

// checkRateLimit prevents panic handler from being overwhelmed
func (ph *PanicHandler) checkRateLimit() bool {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	now := time.Now()

	// Allow max 10 panics per second
	if now.Sub(ph.lastPanicTime) < 100*time.Millisecond {
		return false
	}

	ph.lastPanicTime = now
	return true
}

// beginRecovery starts a recovery session
func (ph *PanicHandler) beginRecovery(r interface{}, depth int32) bool {
	currentState := ph.recoveryState.Load().(*RecoveryState)

	if currentState.IsRecovering {
		return false
	}

	newState := &RecoveryState{
		IsRecovering:   true,
		RecoveryStart:  time.Now(),
		PanicDepth:     depth,
		LastPanicValue: r,
	}

	ph.recoveryState.Store(newState)
	return true
}

// endRecovery ends a recovery session
func (ph *PanicHandler) endRecovery() {
	newState := &RecoveryState{
		IsRecovering:   false,
		RecoveryStart:  time.Time{},
		PanicDepth:     0,
		LastPanicValue: nil,
	}

	ph.recoveryState.Store(newState)
}

// getStackTrace gets stack trace with depth limiting
func (ph *PanicHandler) getStackTrace() []byte {
	// Limit stack trace depth to prevent memory issues
	if ph.config.MaxStackDepth > 0 {
		// For now, use the full stack trace but limit processing
		stack := debug.Stack()

		// If stack is too large, truncate it
		if len(stack) > int(ph.config.MaxStackDepth)*1024 {
			stack = stack[:int(ph.config.MaxStackDepth)*1024]
		}

		return stack
	}

	return debug.Stack()
}

// getCallerInfo returns information about where the panic occurred
func (ph *PanicHandler) getCallerInfo() string {
	var callers [16]uintptr
	n := runtime.Callers(4, callers[:]) // Skip panic handler frames

	if n == 0 {
		return "unknown"
	}

	frames := runtime.CallersFrames(callers[:n])

	// Find the first non-panic handler frame
	for frame, more := frames.Next(); more; frame, more = frames.Next() {
		if !strings.Contains(frame.Function, "panic_handler") &&
			!strings.Contains(frame.Function, "runtime/panic") &&
			!strings.Contains(frame.Function, "runtime/debug") {
			return fmt.Sprintf("%s:%d", frame.File, frame.Line)
		}
	}

	return "unknown"
}

// logSystemInfo logs comprehensive system information
func (ph *PanicHandler) logSystemInfo() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	ph.logger.Error("PANIC SYSTEM INFO",
		zap.Uint64("alloc_mb", m.Alloc/1024/1024),
		zap.Uint64("total_alloc_mb", m.TotalAlloc/1024/1024),
		zap.Uint64("sys_mb", m.Sys/1024/1024),
		zap.Uint32("num_gc", m.NumGC),
		zap.Int("goroutines", runtime.NumGoroutine()),
		zap.Int("cpu_count", runtime.NumCPU()),
	)
}

// executeRecovery executes recovery logic with timeout
func (ph *PanicHandler) executeRecovery(r interface{}, stack []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), ph.config.RecoveryTimeout)
	defer cancel()

	// Log recovery attempt
	ph.logger.Info("Executing panic recovery logic",
		zap.Any("panic_value", r),
		zap.Duration("timeout", ph.config.RecoveryTimeout),
	)

	// Execute recovery in a separate goroutine with timeout
	recoveryDone := make(chan bool, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ph.logger.Error("Recovery logic itself panicked",
					zap.Any("recovery_panic", r),
				)
			}
			recoveryDone <- true
		}()

		// Execute recovery logic here
		ph.performRecoveryActions(r, stack)
	}()

	// Wait for recovery to complete or timeout
	select {
	case <-recoveryDone:
		ph.logger.Info("Panic recovery completed successfully")
	case <-ctx.Done():
		ph.logger.Error("Panic recovery timed out",
			zap.Duration("timeout", ph.config.RecoveryTimeout),
		)
	}
}

// performRecoveryActions performs specific recovery actions
func (ph *PanicHandler) performRecoveryActions(r interface{}, stack []byte) {
	// Log recovery attempt
	ph.logger.Info("Performing recovery actions",
		zap.Any("panic_value", r),
	)

	// Force garbage collection
	ph.logger.Info("Forcing garbage collection")
	runtime.GC()

	// Log memory stats after GC
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	ph.logger.Info("Memory stats after recovery",
		zap.Uint64("alloc_mb", m.Alloc/1024/1024),
		zap.Uint64("total_alloc_mb", m.TotalAlloc/1024/1024),
		zap.Uint64("sys_mb", m.Sys/1024/1024),
	)
}

// HandleFatalError handles fatal errors with comprehensive logging
func (ph *PanicHandler) HandleFatalError(err error, context map[string]interface{}) {
	ph.logger.Error("FATAL ERROR",
		zap.Error(err),
		zap.String("error_type", fmt.Sprintf("%T", err)),
		zap.String("timestamp", time.Now().Format(time.RFC3339)),
		zap.Any("context", context),
	)

	// Log system information
	ph.logSystemInfo()

	// Log stack trace
	stack := debug.Stack()
	ph.logger.Error("FATAL ERROR STACK TRACE",
		zap.String("stack_trace", string(stack)),
	)

	// Exit if configured to do so
	if ph.config.ExitOnFatal {
		ph.logger.Fatal("Application terminating due to fatal error",
			zap.Error(err),
			zap.Int("exit_code", ph.config.ExitCode),
		)
		os.Exit(ph.config.ExitCode)
	}
}

// LogError logs errors with comprehensive context
func (ph *PanicHandler) LogError(err error, context map[string]interface{}) {
	ph.logger.Error("ERROR",
		zap.Error(err),
		zap.String("error_type", fmt.Sprintf("%T", err)),
		zap.String("timestamp", time.Now().Format(time.RFC3339)),
		zap.Any("context", context),
	)

	// Log stack trace for errors if enabled
	if ph.config.LogStackTraces {
		stack := debug.Stack()
		ph.logger.Error("ERROR STACK TRACE",
			zap.String("stack_trace", string(stack)),
		)
	}
}

// GetConfig returns a copy of the current configuration
func (ph *PanicHandler) GetConfig() PanicHandlerConfig {
	return *ph.config
}

// UpdateConfig updates the panic handler configuration
func (ph *PanicHandler) UpdateConfig(config *PanicHandlerConfig) {
	if config != nil {
		ph.config = config
		ph.logger.Info("Panic handler configuration updated",
			zap.Bool("enable_recovery", ph.config.EnableRecovery),
			zap.Bool("log_stack_traces", ph.config.LogStackTraces),
			zap.Bool("exit_on_fatal", ph.config.ExitOnFatal),
		)
	}
}

// GetEnvironment returns the current environment
func GetEnvironment() string {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = os.Getenv("ENVIRONMENT")
	}
	if env == "" {
		env = "development"
	}
	return strings.ToLower(env)
}

// LoadPanicHandlerConfig loads panic handler configuration from file and environment
func LoadPanicHandlerConfig(configPath string, environment string) (*PanicHandlerConfig, error) {
	// Start with defaults
	config := DefaultPanicHandlerConfig()

	// Try to load from YAML file if it exists
	if _, err := os.Stat(configPath); err == nil {
		if err := loadFromYAML(config, configPath, environment); err != nil {
			// Log warning but continue with defaults
			fmt.Printf("Warning: Failed to load panic handler config from %s: %v\n", configPath, err)
		}
	}

	// Override with environment variables
	loadFromEnvironment(config)

	return config, nil
}

// loadFromYAML loads configuration from YAML file
func loadFromYAML(config *PanicHandlerConfig, configPath string, environment string) error {
	// Read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var yamlConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Get environment-specific config
	envConfig, exists := yamlConfig[environment]
	if !exists {
		// Fall back to default if environment-specific config doesn't exist
		envConfig = yamlConfig["default"]
	}

	if envConfig == nil {
		return fmt.Errorf("no configuration found for environment '%s' or 'default'", environment)
	}

	// Convert to map
	envMap, ok := envConfig.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid configuration format for environment '%s'", environment)
	}

	// Apply configuration values
	if val, exists := envMap["enable_recovery"]; exists {
		if boolVal, ok := val.(bool); ok {
			config.EnableRecovery = boolVal
		}
	}

	if val, exists := envMap["log_stack_traces"]; exists {
		if boolVal, ok := val.(bool); ok {
			config.LogStackTraces = boolVal
		}
	}

	if val, exists := envMap["log_goroutine_info"]; exists {
		if boolVal, ok := val.(bool); ok {
			config.LogGoroutineInfo = boolVal
		}
	}

	if val, exists := envMap["max_stack_depth"]; exists {
		if intVal, ok := val.(int); ok {
			config.MaxStackDepth = intVal
		}
	}

	if val, exists := envMap["recovery_timeout"]; exists {
		if strVal, ok := val.(string); ok {
			if duration, err := time.ParseDuration(strVal); err == nil {
				config.RecoveryTimeout = duration
			}
		}
	}

	if val, exists := envMap["exit_on_fatal"]; exists {
		if boolVal, ok := val.(bool); ok {
			config.ExitOnFatal = boolVal
		}
	}

	if val, exists := envMap["exit_code"]; exists {
		if intVal, ok := val.(int); ok {
			config.ExitCode = intVal
		}
	}

	return nil
}

// loadFromEnvironment loads configuration from environment variables
func loadFromEnvironment(config *PanicHandlerConfig) {
	// EnableRecovery
	if val := os.Getenv("PANIC_ENABLE_RECOVERY"); val != "" {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			config.EnableRecovery = boolVal
		}
	}

	// LogStackTraces
	if val := os.Getenv("PANIC_LOG_STACK_TRACES"); val != "" {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			config.LogStackTraces = boolVal
		}
	}

	// LogGoroutineInfo
	if val := os.Getenv("PANIC_LOG_GOROUTINE_INFO"); val != "" {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			config.LogGoroutineInfo = boolVal
		}
	}

	// MaxStackDepth
	if val := os.Getenv("PANIC_MAX_STACK_DEPTH"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.MaxStackDepth = intVal
		}
	}

	// RecoveryTimeout
	if val := os.Getenv("PANIC_RECOVERY_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.RecoveryTimeout = duration
		}
	}

	// ExitOnFatal
	if val := os.Getenv("PANIC_EXIT_ON_FATAL"); val != "" {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			config.ExitOnFatal = boolVal
		}
	}

	// ExitCode
	if val := os.Getenv("PANIC_EXIT_CODE"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.ExitCode = intVal
		}
	}
}

// Validate validates the configuration
func (c *PanicHandlerConfig) Validate() error {
	if c.MaxStackDepth <= 0 {
		return fmt.Errorf("max_stack_depth must be positive, got %d", c.MaxStackDepth)
	}

	if c.RecoveryTimeout <= 0 {
		return fmt.Errorf("recovery_timeout must be positive, got %v", c.RecoveryTimeout)
	}

	if c.ExitCode < 0 || c.ExitCode > 255 {
		return fmt.Errorf("exit_code must be between 0 and 255, got %d", c.ExitCode)
	}

	return nil
}

// String returns a string representation of the configuration
func (c *PanicHandlerConfig) String() string {
	return fmt.Sprintf(
		"PanicHandlerConfig{EnableRecovery: %t, LogStackTraces: %t, LogGoroutineInfo: %t, "+
			"MaxStackDepth: %d, RecoveryTimeout: %v, ExitOnFatal: %t, ExitCode: %d}",
		c.EnableRecovery, c.LogStackTraces, c.LogGoroutineInfo,
		c.MaxStackDepth, c.RecoveryTimeout, c.ExitOnFatal, c.ExitCode,
	)
}

// GetStatus returns the current status of the panic handler
func (ph *PanicHandler) GetStatus() map[string]interface{} {
	state := ph.recoveryState.Load().(*RecoveryState)

	return map[string]interface{}{
		"panic_depth":       atomic.LoadInt32(&ph.panicDepth),
		"max_panic_depth":   ph.maxPanicDepth,
		"total_panic_count": atomic.LoadInt64(&ph.panicCount),
		"is_recovering":     state.IsRecovering,
		"recovery_start":    state.RecoveryStart,
		"last_panic_time":   ph.lastPanicTime,
		"last_panic_value":  state.LastPanicValue,
		"config":            ph.config,
	}
}

// GetHealth returns the health status of the panic handler
func (ph *PanicHandler) GetHealth() string {
	state := ph.recoveryState.Load().(*RecoveryState)
	currentDepth := atomic.LoadInt32(&ph.panicDepth)

	if currentDepth > ph.maxPanicDepth {
		return "CRITICAL"
	}

	if state.IsRecovering {
		return "RECOVERING"
	}

	if currentDepth > 0 {
		return "WARNING"
	}

	return "HEALTHY"
}

// IsHealthy checks if the panic handler is in a healthy state
func (ph *PanicHandler) IsHealthy() bool {
	return ph.GetHealth() == "HEALTHY"
}

// startMemoryMonitoring starts background memory monitoring
func (ph *PanicHandler) startMemoryMonitoring() {
	if !ph.config.EnableMemoryMonitoring {
		return
	}

	go func() {
		ticker := time.NewTicker(ph.config.MemoryCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ph.checkMemoryUsage()
			}
		}
	}()
}

// checkMemoryUsage checks current memory usage and triggers cleanup if needed
func (ph *PanicHandler) checkMemoryUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	currentUsageMB := m.Alloc / 1024 / 1024

	// Log memory usage if monitoring is enabled
	if ph.config.EnableMemoryMonitoring {
		ph.logger.Debug("Memory usage check",
			zap.Uint64("alloc_mb", currentUsageMB),
			zap.Uint64("total_alloc_mb", m.TotalAlloc/1024/1024),
			zap.Uint64("sys_mb", m.Sys/1024/1024),
			zap.Uint64("threshold_mb", ph.config.MemoryCleanupThreshold),
		)
	}

	// Trigger cleanup if threshold exceeded
	if currentUsageMB > ph.config.MemoryCleanupThreshold {
		ph.logger.Warn("Memory usage threshold exceeded - triggering cleanup",
			zap.Uint64("current_mb", currentUsageMB),
			zap.Uint64("threshold_mb", ph.config.MemoryCleanupThreshold),
		)
		ph.performMemoryCleanup()
	}

	// Emergency exit if max memory usage exceeded
	if currentUsageMB > ph.config.MaxMemoryUsage {
		ph.logger.Error("CRITICAL: Maximum memory usage exceeded - emergency exit",
			zap.Uint64("current_mb", currentUsageMB),
			zap.Uint64("max_mb", ph.config.MaxMemoryUsage),
		)
		os.Exit(3) // Exit code 3 for memory exhaustion
	}
}

// performMemoryCleanup performs aggressive memory cleanup
func (ph *PanicHandler) performMemoryCleanup() {
	// Force garbage collection
	runtime.GC()

	// Force memory release to OS
	debug.FreeOSMemory()

	// Log cleanup results
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	ph.logger.Info("Memory cleanup completed",
		zap.Uint64("alloc_mb_after", m.Alloc/1024/1024),
		zap.Uint64("sys_mb_after", m.Sys/1024/1024),
	)
}

// getMemoryStats returns current memory statistics
func (ph *PanicHandler) getMemoryStats() runtime.MemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m
}

// AddAlertChannel adds an alert channel for notifications
func (ph *PanicHandler) AddAlertChannel(channel AlertChannel) {
	ph.alertMutex.Lock()
	defer ph.alertMutex.Unlock()

	ph.alertChannels = append(ph.alertChannels, channel)
	ph.logger.Info("Alert channel added",
		zap.String("channel_name", channel.GetName()),
	)
}

// RemoveAlertChannel removes an alert channel
func (ph *PanicHandler) RemoveAlertChannel(channelName string) {
	ph.alertMutex.Lock()
	defer ph.alertMutex.Unlock()

	for i, channel := range ph.alertChannels {
		if channel.GetName() == channelName {
			ph.alertChannels = append(ph.alertChannels[:i], ph.alertChannels[i+1:]...)
			ph.logger.Info("Alert channel removed",
				zap.String("channel_name", channelName),
			)
			return
		}
	}
}

// sendAlert sends an alert through all configured channels
func (ph *PanicHandler) sendAlert(alert *PanicAlert) {
	ph.alertMutex.RLock()
	channels := make([]AlertChannel, len(ph.alertChannels))
	copy(channels, ph.alertChannels)
	ph.alertMutex.RUnlock()

	for _, channel := range channels {
		go func(ch AlertChannel) {
			if err := ch.SendAlert(alert); err != nil {
				ph.logger.Error("Failed to send alert",
					zap.String("channel_name", ch.GetName()),
					zap.Error(err),
				)
			}
		}(channel)
	}
}

// checkAlertConditions checks if alert conditions are met
func (ph *PanicHandler) checkAlertConditions(r interface{}, depth int32) {
	totalPanics := atomic.LoadInt64(&ph.panicCount)

	// Get current memory usage
	memStats := ph.getMemoryStats()
	currentMemoryMB := memStats.Alloc / 1024 / 1024

	// Check for critical conditions
	if depth > ph.maxPanicDepth {
		ph.sendCriticalAlert(r, depth, totalPanics, currentMemoryMB)
		return
	}

	// Check for high panic rate
	if ph.isHighPanicRate() {
		ph.sendHighRateAlert(r, depth, totalPanics, currentMemoryMB)
		return
	}

	// Check for memory issues
	if currentMemoryMB > ph.config.MemoryCleanupThreshold {
		ph.sendMemoryAlert(r, depth, totalPanics, currentMemoryMB)
		return
	}

	// Check for unusual panic patterns
	if ph.isUnusualPanicPattern(r) {
		ph.sendPatternAlert(r, depth, totalPanics, currentMemoryMB)
		return
	}
}

// isHighPanicRate checks if panic rate is unusually high
func (ph *PanicHandler) isHighPanicRate() bool {
	// Simple rate check - could be enhanced with sliding window
	ph.mu.RLock()
	defer ph.mu.RUnlock()

	if ph.lastPanicTime.IsZero() {
		return false
	}

	// Safety check to prevent division by zero
	if ph.config.MaxPanicsPerSecond <= 0 {
		return false
	}

	timeSinceLastPanic := time.Since(ph.lastPanicTime)
	return timeSinceLastPanic < time.Second/time.Duration(ph.config.MaxPanicsPerSecond)
}

// isUnusualPanicPattern checks for unusual panic patterns
func (ph *PanicHandler) isUnusualPanicPattern(r interface{}) bool {
	// Add panic to history
	ph.addToHistory(r)

	// Check for repeated panics of the same type
	recentPanics := ph.getRecentPanics(1 * time.Minute)

	// Count panics of the same type
	panicTypeCount := make(map[string]int)
	for _, panic := range recentPanics {
		panicType := fmt.Sprintf("%T", panic.Value)
		panicTypeCount[panicType]++
	}

	// Alert if any panic type occurs more than 5 times in a minute
	for panicType, count := range panicTypeCount {
		if count > 5 {
			ph.logger.Warn("Unusual panic pattern detected",
				zap.String("panic_type", panicType),
				zap.Int("count", count),
				zap.Duration("time_window", time.Minute),
			)
			return true
		}
	}

	return false
}

// addToHistory adds a panic event to history
func (ph *PanicHandler) addToHistory(r interface{}) {
	ph.alertMutex.Lock()
	defer ph.alertMutex.Unlock()

	memStats := ph.getMemoryStats()
	event := PanicEvent{
		Timestamp:   time.Now(),
		Value:       r,
		Type:        fmt.Sprintf("%T", r),
		Depth:       atomic.LoadInt32(&ph.panicDepth),
		MemoryUsage: memStats.Alloc / 1024 / 1024,
	}

	ph.panicHistory = append(ph.panicHistory, event)

	// Keep only last 100 events to prevent memory growth
	if len(ph.panicHistory) > 100 {
		ph.panicHistory = ph.panicHistory[1:]
	}
}

// getRecentPanics returns panics from the last specified duration
func (ph *PanicHandler) getRecentPanics(duration time.Duration) []PanicEvent {
	ph.alertMutex.RLock()
	defer ph.alertMutex.RUnlock()

	cutoff := time.Now().Add(-duration)
	var recent []PanicEvent

	for _, event := range ph.panicHistory {
		if event.Timestamp.After(cutoff) {
			recent = append(recent, event)
		}
	}

	return recent
}

// sendCriticalAlert sends a critical alert
func (ph *PanicHandler) sendCriticalAlert(r interface{}, depth int32, totalPanics int64, memoryMB uint64) {
	alert := &PanicAlert{
		Severity:    "CRITICAL",
		Message:     "Critical panic depth exceeded - system at risk",
		Timestamp:   time.Now(),
		PanicValue:  r,
		PanicType:   fmt.Sprintf("%T", r),
		PanicDepth:  depth,
		TotalPanics: totalPanics,
		MemoryUsage: memoryMB,
		Context: map[string]interface{}{
			"max_depth": ph.maxPanicDepth,
		},
	}

	ph.sendAlert(alert)
}

// sendHighRateAlert sends a high panic rate alert
func (ph *PanicHandler) sendHighRateAlert(r interface{}, depth int32, totalPanics int64, memoryMB uint64) {
	alert := &PanicAlert{
		Severity:    "HIGH",
		Message:     "High panic rate detected - system under stress",
		Timestamp:   time.Now(),
		PanicValue:  r,
		PanicType:   fmt.Sprintf("%T", r),
		PanicDepth:  depth,
		TotalPanics: totalPanics,
		MemoryUsage: memoryMB,
		Context: map[string]interface{}{
			"max_panics_per_second": ph.config.MaxPanicsPerSecond,
		},
	}

	ph.sendAlert(alert)
}

// sendMemoryAlert sends a memory usage alert
func (ph *PanicHandler) sendMemoryAlert(r interface{}, depth int32, totalPanics int64, memoryMB uint64) {
	alert := &PanicAlert{
		Severity:    "MEDIUM",
		Message:     "Memory usage threshold exceeded",
		Timestamp:   time.Now(),
		PanicValue:  r,
		PanicType:   fmt.Sprintf("%T", r),
		PanicDepth:  depth,
		TotalPanics: totalPanics,
		MemoryUsage: memoryMB,
		Context: map[string]interface{}{
			"threshold_mb": ph.config.MemoryCleanupThreshold,
			"max_mb":       ph.config.MaxMemoryUsage,
		},
	}

	ph.sendAlert(alert)
}

// sendPatternAlert sends an unusual pattern alert
func (ph *PanicHandler) sendPatternAlert(r interface{}, depth int32, totalPanics int64, memoryMB uint64) {
	alert := &PanicAlert{
		Severity:    "LOW",
		Message:     "Unusual panic pattern detected",
		Timestamp:   time.Now(),
		PanicValue:  r,
		PanicType:   fmt.Sprintf("%T", r),
		PanicDepth:  depth,
		TotalPanics: totalPanics,
		MemoryUsage: memoryMB,
		Context: map[string]interface{}{
			"pattern_type": "repeated_panics",
		},
	}

	ph.sendAlert(alert)
}

// startBackgroundWorkers starts background workers for async panic processing
func (ph *PanicHandler) startBackgroundWorkers() {
	// Start panic processing workers
	for i := 0; i < 10; i++ {
		go ph.panicWorker(i)
	}

	// Start metrics cache updater
	go ph.metricsCacheUpdater()

	// Start panic queue processor
	go ph.panicQueueProcessor()
}

// panicWorker processes panics from the queue
func (ph *PanicHandler) panicWorker(id int) {
	for {
		select {
		case <-ph.stopChan:
			return
		case panicEvent := <-ph.panicQueue:
			ph.processPanicAsync(panicEvent, id)
		}
	}
}

// processPanicAsync processes a panic asynchronously
func (ph *PanicHandler) processPanicAsync(event *PanicEvent, workerID int) {
	// Acquire worker from pool
	select {
	case ph.workerPool <- struct{}{}:
		defer func() { <-ph.workerPool }()
	default:
		// No workers available, process directly
	}

	// Process panic with timeout
	done := make(chan bool, 1)
	go func() {
		ph.processPanicEvent(event)
		done <- true
	}()

	select {
	case <-done:
		// Processing completed successfully
	case <-time.After(ph.config.RecoveryTimeout):
		ph.logger.Warn("Panic processing timeout",
			zap.Int("worker_id", workerID),
			zap.Any("panic_value", event.Value),
			zap.Duration("timeout", ph.config.RecoveryTimeout),
		)
	}
}

// processPanicEvent processes a single panic event
func (ph *PanicHandler) processPanicEvent(event *PanicEvent) {
	// Update metrics cache
	ph.updateMetricsCache()

	// Add to history (already done in addToHistory)
	// Check alert conditions
	ph.checkAlertConditions(event.Value, event.Depth)
}

// metricsCacheUpdater updates the metrics cache periodically
func (ph *PanicHandler) metricsCacheUpdater() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ph.stopChan:
			return
		case <-ticker.C:
			ph.updateMetricsCache()
		}
	}
}

// updateMetricsCache updates the cached metrics
func (ph *PanicHandler) updateMetricsCache() {
	ph.metricsCache.mu.Lock()
	defer ph.metricsCache.mu.Unlock()

	// Update memory stats
	runtime.ReadMemStats(&ph.metricsCache.memoryStats)

	// Update panic count
	ph.metricsCache.panicCount = atomic.LoadInt64(&ph.panicCount)

	// Update recovery state
	ph.metricsCache.recoveryState = ph.recoveryState.Load().(*RecoveryState)

	// Update timestamp
	ph.metricsCache.lastUpdate = time.Now()
}

// panicQueueProcessor processes panics from the queue with batching
func (ph *PanicHandler) panicQueueProcessor() {
	batch := make([]*PanicEvent, 0, 100)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ph.stopChan:
			// Process remaining batch
			if len(batch) > 0 {
				ph.processPanicBatch(batch)
			}
			return
		case event := <-ph.panicQueue:
			batch = append(batch, event)

			// Process batch if full or on timer
			if len(batch) >= 100 {
				ph.processPanicBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			// Process batch on timer
			if len(batch) > 0 {
				ph.processPanicBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

// processPanicBatch processes a batch of panics efficiently
func (ph *PanicHandler) processPanicBatch(batch []*PanicEvent) {
	if len(batch) == 0 {
		return
	}

	// Process batch in parallel
	semaphore := make(chan struct{}, 5) // Max 5 concurrent batch items
	var wg sync.WaitGroup

	for _, event := range batch {
		wg.Add(1)
		go func(evt *PanicEvent) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
				ph.processPanicEvent(evt)
			default:
				// Process directly if semaphore is full
				ph.processPanicEvent(evt)
			}
		}(event)
	}

	wg.Wait()
}

// Shutdown gracefully shuts down the panic handler
func (ph *PanicHandler) Shutdown() {
	close(ph.stopChan)

	// Wait for background workers to finish
	time.Sleep(100 * time.Millisecond)

	ph.logger.Info("Panic handler shutdown completed")
}

// GetCachedStatus returns cached status information for better performance
func (ph *PanicHandler) GetCachedStatus() map[string]interface{} {
	ph.metricsCache.mu.RLock()
	defer ph.metricsCache.mu.RUnlock()

	state := ph.metricsCache.recoveryState
	if state == nil {
		state = &RecoveryState{}
	}

	return map[string]interface{}{
		"panic_depth":       atomic.LoadInt32(&ph.panicDepth),
		"max_panic_depth":   ph.maxPanicDepth,
		"total_panic_count": ph.metricsCache.panicCount,
		"is_recovering":     state.IsRecovering,
		"recovery_start":    state.RecoveryStart,
		"last_panic_time":   ph.lastPanicTime,
		"last_panic_value":  state.LastPanicValue,
		"memory_alloc_mb":   ph.metricsCache.memoryStats.Alloc / 1024 / 1024,
		"memory_sys_mb":     ph.metricsCache.memoryStats.Sys / 1024 / 1024,
		"cache_last_update": ph.metricsCache.lastUpdate,
		"queue_length":      len(ph.panicQueue),
		"worker_pool_size":  len(ph.workerPool),
		"config":            ph.config,
	}
}

// GetPerformanceMetrics returns performance-related metrics
func (ph *PanicHandler) GetPerformanceMetrics() map[string]interface{} {
	ph.metricsCache.mu.RLock()
	defer ph.metricsCache.mu.RUnlock()

	return map[string]interface{}{
		"queue_capacity":        cap(ph.panicQueue),
		"queue_length":          len(ph.panicQueue),
		"worker_pool_capacity":  cap(ph.workerPool),
		"worker_pool_available": len(ph.workerPool),
		"cache_last_update":     ph.metricsCache.lastUpdate,
		"memory_alloc_mb":       ph.metricsCache.memoryStats.Alloc / 1024 / 1024,
		"memory_total_alloc_mb": ph.metricsCache.memoryStats.TotalAlloc / 1024 / 1024,
		"memory_sys_mb":         ph.metricsCache.memoryStats.Sys / 1024 / 1024,
		"goroutines":            runtime.NumGoroutine(),
	}
}

// ConsoleAlertChannel implements AlertChannel for console output
type ConsoleAlertChannel struct {
	name string
}

// NewConsoleAlertChannel creates a new console alert channel
func NewConsoleAlertChannel(name string) *ConsoleAlertChannel {
	return &ConsoleAlertChannel{name: name}
}

// SendAlert sends an alert to the console
func (c *ConsoleAlertChannel) SendAlert(alert *PanicAlert) error {
	fmt.Printf("[%s] %s ALERT: %s\n",
		c.name,
		alert.Severity,
		alert.Message,
	)
	fmt.Printf("  Panic Type: %s\n", alert.PanicType)
	fmt.Printf("  Panic Depth: %d\n", alert.PanicDepth)
	fmt.Printf("  Total Panics: %d\n", alert.TotalPanics)
	fmt.Printf("  Memory Usage: %d MB\n", alert.MemoryUsage)
	fmt.Printf("  Timestamp: %s\n", alert.Timestamp.Format(time.RFC3339))
	if len(alert.Context) > 0 {
		fmt.Printf("  Context: %+v\n", alert.Context)
	}
	fmt.Println()
	return nil
}

// GetName returns the channel name
func (c *ConsoleAlertChannel) GetName() string {
	return c.name
}
