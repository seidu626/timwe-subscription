package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"math"

	"github.com/fsnotify/fsnotify"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// BatchOptinRequest represents the request structure
type BatchOptinRequest struct {
	Count        int      `json:"count"`
	EntryChannel string   `json:"entry_channel"`
	MSISDNS      []string `json:"msisdns"`
	ProductIds   []string `json:"product_ids"`
	Telco        string   `json:"telco"`
}

// BatchOptinResponse represents the response structure
type BatchOptinResponse struct {
	Total        int                     `json:"total"`
	Successful   int                     `json:"successful"`
	Failed       int                     `json:"failed"`
	ErrorDetails *map[string]interface{} `json:"errorDetails,omitempty"`
}

// Batch job async response from POST
type batchJobEnqueueResponse struct {
	JobID string `json:"jobId"`
}

// Batch job status polled from GET
type batchJobState string

const (
	jobStatePending   batchJobState = "pending"
	jobStateRunning   batchJobState = "running"
	jobStateCompleted batchJobState = "completed"
	jobStateFailed    batchJobState = "failed"
)

type batchJobStatus struct {
	ID           string                 `json:"id"`
	State        batchJobState          `json:"state"`
	Total        int                    `json:"total"`
	Processed    int64                  `json:"processed"`
	Successful   int64                  `json:"successful"`
	Failed       int64                  `json:"failed"`
	ErrorDetails map[string]interface{} `json:"errorDetails,omitempty"`
	StartedAt    time.Time              `json:"startedAt"`
	CompletedAt  *time.Time             `json:"completedAt,omitempty"`
}

// PauseWindow specifies a daily pause interval in local or configured timezone
// Start/End format: "HH:MM" (24h). Windows may wrap midnight, e.g. 22:00-06:00.
type PauseWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type parsedPauseWindow struct {
	startMinutes int // minutes since midnight
	endMinutes   int // minutes since midnight
}

// ProcessorConfig holds the configuration for the batch processor
type ProcessorConfig struct {
	BaseURL          string   `json:"base_url"`
	StartCount       int      `json:"start_count"`
	MaxCount         int      `json:"max_count"`
	Increment        int      `json:"increment"`
	Telco            string   `json:"telco"`
	EntryChannel     string   `json:"entry_channel"`            // Single channel (legacy support)
	EntryChannels    []string `json:"entry_channels,omitempty"` // Multiple channels for rotation
	ProductIds       []string `json:"product_ids"`
	WaitBetweenCalls string   `json:"wait_between_calls"` // String for JSON unmarshaling
	MaxRetries       int      `json:"max_retries"`
	RetryDelay       string   `json:"retry_delay"` // String for JSON unmarshaling
	PollInterval     string   `json:"poll_interval"`

	// Metrics
	EnableMetrics bool   `json:"enable_metrics"`
	MetricsAddr   string `json:"metrics_addr"` // e.g. ":9101"

	// Pause/resume schedule
	PauseWindows []PauseWindow `json:"pause_windows,omitempty"`
	Timezone     string        `json:"timezone,omitempty"` // IANA TZ, e.g. "Africa/Accra"; empty means local

	// Safety: max time to poll a single job before failing
	MaxPollingDuration string `json:"max_polling_duration,omitempty"`

	// Progress tracking
	SaveProgressInterval string `json:"save_progress_interval,omitempty"` // How often to save progress
	ResumeFromProgress   bool   `json:"resume_from_progress,omitempty"`   // Resume from last saved progress

	// Continuous processing
	ContinuousMode bool   `json:"continuous_mode,omitempty"` // Restart from start after completion
	RestartDelay   string `json:"restart_delay,omitempty"`   // Delay before restart (e.g., "5m")
	MaxRestarts    int    `json:"max_restarts,omitempty"`    // Maximum number of restarts (0 = unlimited)

	// Parsed durations (not exported)
	waitDuration         time.Duration
	retryDuration        time.Duration
	pollDuration         time.Duration
	maxPollDur           time.Duration
	progressSaveInterval time.Duration
	restartDelay         time.Duration

	// Derived
	location           *time.Location
	parsedPauseWindows []parsedPauseWindow

	// Entry channel rotation
	currentChannelIndex int
	channelMutex        sync.Mutex

	// Config file path for hot reloading
	configFilePath string
	lastModified   time.Time
}

// BatchProcessor handles the batch processing logic
type BatchProcessor struct {
	config          *ProcessorConfig
	logger          *zap.Logger
	httpClient      *http.Client
	stopChan        chan struct{}
	wg              sync.WaitGroup
	ctx             context.Context
	cancel          context.CancelFunc
	metricsEnabled  bool
	metricsServer   *http.Server
	configMutex     sync.RWMutex
	circuitBreaker  *CircuitBreaker
	healthCheck     *HealthChecker
	progressTracker *ProgressTracker
}

// UpdateConfig updates the processor configuration (thread-safe)
func (bp *BatchProcessor) UpdateConfig(newConfig *ProcessorConfig) {
	bp.configMutex.Lock()
	defer bp.configMutex.Unlock()

	oldConfig := bp.config
	bp.config = newConfig

	bp.logger.Info("Configuration updated via hot reload",
		zap.String("oldEntryChannel", oldConfig.EntryChannel),
		zap.String("newEntryChannel", newConfig.EntryChannel),
		zap.Strings("oldEntryChannels", oldConfig.EntryChannels),
		zap.Strings("newEntryChannels", newConfig.EntryChannels),
		zap.Duration("oldWaitTime", oldConfig.GetWaitDuration()),
		zap.Duration("newWaitTime", newConfig.GetWaitDuration()),
		zap.Duration("oldPollInterval", oldConfig.pollDuration),
		zap.Duration("newPollInterval", newConfig.pollDuration),
	)
}

// GetConfig returns a thread-safe copy of the current configuration
func (bp *BatchProcessor) GetConfig() *ProcessorConfig {
	bp.configMutex.RLock()
	defer bp.configMutex.RUnlock()
	return bp.config
}

// ConfigWatcher handles configuration file watching and hot reloading
type ConfigWatcher struct {
	watcher       *fsnotify.Watcher
	configPath    string
	logger        *zap.Logger
	reloadChan    chan *ProcessorConfig
	stopChan      chan struct{}
	wg            sync.WaitGroup
	mu            sync.RWMutex
	currentConfig *ProcessorConfig
}

// NewConfigWatcher creates a new configuration watcher
func NewConfigWatcher(configPath string, logger *zap.Logger) (*ConfigWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &ConfigWatcher{
		watcher:    watcher,
		configPath: configPath,
		logger:     logger,
		reloadChan: make(chan *ProcessorConfig, 1),
		stopChan:   make(chan struct{}),
	}, nil
}

// Start begins watching the configuration file for changes
func (cw *ConfigWatcher) Start() error {
	// Get absolute path for the config file
	absConfigPath, err := filepath.Abs(cw.configPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for config file: %w", err)
	}
	cw.configPath = absConfigPath

	// Add the config file directory to the watcher
	configDir := filepath.Dir(absConfigPath)

	cw.logger.Info("Setting up file watcher",
		zap.String("configFile", absConfigPath),
		zap.String("configDir", configDir))

	if err := cw.watcher.Add(configDir); err != nil {
		return fmt.Errorf("failed to watch config directory: %w", err)
	}

	// Also try to watch the specific file directly (some systems support this)
	if err := cw.watcher.Add(absConfigPath); err != nil {
		cw.logger.Debug("Could not watch config file directly, watching directory only",
			zap.String("file", absConfigPath), zap.Error(err))
	}

	cw.wg.Add(1)
	go cw.watchLoop()

	cw.logger.Info("Configuration watcher started",
		zap.String("configPath", cw.configPath),
		zap.String("configDir", configDir))
	return nil
}

// ListWatchedFiles returns information about what files are being watched
func (cw *ConfigWatcher) ListWatchedFiles() []string {
	// Note: fsnotify doesn't provide a direct way to list watched files
	// This is a workaround to show what we're trying to watch
	return []string{cw.configPath, filepath.Dir(cw.configPath)}
}

// Stop stops the configuration watcher
func (cw *ConfigWatcher) Stop() {
	close(cw.stopChan)
	cw.watcher.Close()
	cw.wg.Wait()
	cw.logger.Info("Configuration watcher stopped")
}

// watchLoop monitors the configuration file for changes
func (cw *ConfigWatcher) watchLoop() {
	defer cw.wg.Done()

	for {
		select {
		case <-cw.stopChan:
			return
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}
			cw.handleFileEvent(event)
		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			cw.logger.Error("File watcher error", zap.Error(err))
		}
	}
}

// handleFileEvent processes file system events
func (cw *ConfigWatcher) handleFileEvent(event fsnotify.Event) {
	// Get absolute path of the event file
	eventPath, err := filepath.Abs(event.Name)
	if err != nil {
		cw.logger.Debug("Could not get absolute path for event", zap.String("event", event.Name), zap.Error(err))
		return
	}

	// Log all events if debug is enabled
	cw.logger.Debug("File event received",
		zap.String("eventFile", eventPath),
		zap.String("configFile", cw.configPath),
		zap.String("operation", event.Op.String()))

	// Check if this event is for our config file
	if eventPath != cw.configPath {
		cw.logger.Debug("Ignoring event for different file",
			zap.String("eventFile", eventPath),
			zap.String("configFile", cw.configPath))
		return
	}

	cw.logger.Info("Configuration file event detected",
		zap.String("file", eventPath),
		zap.String("operation", event.Op.String()))

	// Debounce rapid file changes
	time.Sleep(100 * time.Millisecond)

	// Check if the file was modified
	if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
		cw.logger.Info("Configuration file changed, attempting to reload",
			zap.String("file", eventPath), zap.String("operation", event.Op.String()))

		if err := cw.reloadConfig(); err != nil {
			cw.logger.Error("Failed to reload configuration", zap.Error(err))
			return
		}

		cw.logger.Info("Configuration reloaded successfully")
	}
}

// reloadConfig loads and validates the configuration file
func (cw *ConfigWatcher) reloadConfig() error {
	cw.logger.Info("Starting configuration reload", zap.String("configFile", cw.configPath))

	config, err := loadConfig(cw.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Validate the new configuration
	if err := cw.validateConfig(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Update the current configuration
	cw.mu.Lock()
	oldConfig := cw.currentConfig
	cw.currentConfig = config
	cw.mu.Unlock()

	cw.logger.Info("Configuration loaded successfully",
		zap.String("oldEntryChannel", oldConfig.EntryChannel),
		zap.String("newEntryChannel", config.EntryChannel))

	// Send the new configuration to the reload channel
	select {
	case cw.reloadChan <- config:
		cw.logger.Info("Configuration sent to reload channel")
	default:
		// Channel is full, log warning
		cw.logger.Warn("Configuration reload channel is full, dropping reload event")
	}

	return nil
}

// ManualReload manually triggers a configuration reload (useful for testing)
func (cw *ConfigWatcher) ManualReload() error {
	cw.logger.Info("Manual configuration reload triggered")
	return cw.reloadConfig()
}

// validateConfig validates the configuration before applying it
func (cw *ConfigWatcher) validateConfig(config *ProcessorConfig) error {
	// Basic validation - ensure required fields are set
	if config.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}
	if config.StartCount <= 0 {
		return fmt.Errorf("start_count must be positive")
	}
	if config.MaxCount <= 0 {
		return fmt.Errorf("max_count must be positive")
	}
	if config.Increment <= 0 {
		return fmt.Errorf("increment must be positive")
	}
	if config.Telco == "" {
		return fmt.Errorf("telco is required")
	}
	if len(config.ProductIds) == 0 {
		return fmt.Errorf("product_ids cannot be empty")
	}

	return nil
}

// GetCurrentConfig returns the current configuration
func (cw *ConfigWatcher) GetCurrentConfig() *ProcessorConfig {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.currentConfig
}

// GetReloadChannel returns the channel for receiving configuration reload events
func (cw *ConfigWatcher) GetReloadChannel() <-chan *ProcessorConfig {
	return cw.reloadChan
}

// Prometheus metrics
var (
	metricBatchesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "batch_processor_batches_total",
			Help: "Total number of processed batches by outcome",
		},
		[]string{"outcome"},
	)
	metricBatchDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "batch_processor_batch_duration_seconds",
			Help:    "Duration of batch processing in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"outcome"},
	)
	metricRetriesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "batch_processor_retries_total",
			Help: "Total number of retry attempts across all batches",
		},
	)
	metricCurrentCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "batch_processor_current_count",
			Help: "Current count value being processed",
		},
	)
	metricPaused = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "batch_processor_paused",
			Help: "Whether the processor is currently paused (1) or running (0)",
		},
	)
)

func registerMetricsOnce() {
	// Register may panic if duplicate; use MustRegister guarded by recover in case already registered by multiple runs
	// However in a single process, this will be called once.
	prometheus.MustRegister(metricBatchesTotal)
	prometheus.MustRegister(metricBatchDuration)
	prometheus.MustRegister(metricRetriesTotal)
	prometheus.MustRegister(metricCurrentCount)
	prometheus.MustRegister(metricPaused)
}

// GetWaitDuration returns the parsed wait duration
func (c *ProcessorConfig) GetWaitDuration() time.Duration {
	return c.waitDuration
}

// GetNextEntryChannel returns the next entry channel in rotation
func (c *ProcessorConfig) GetNextEntryChannel() string {
	c.channelMutex.Lock()
	defer c.channelMutex.Unlock()

	// If no channels configured, fall back to single channel
	if len(c.EntryChannels) == 0 {
		return c.EntryChannel
	}

	// Get current channel and advance index
	channel := c.EntryChannels[c.currentChannelIndex]
	c.currentChannelIndex = (c.currentChannelIndex + 1) % len(c.EntryChannels)

	return channel
}

// GetRetryDuration returns the parsed retry duration
func (c *ProcessorConfig) GetRetryDuration() time.Duration {
	return c.retryDuration
}

// NewBatchProcessor creates a new batch processor instance
func NewBatchProcessor(config *ProcessorConfig, logger *zap.Logger) *BatchProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	bp := &BatchProcessor{
		config:          config,
		logger:          logger,
		httpClient:      &http.Client{Timeout: 5 * time.Minute}, // Long timeout for large batches
		stopChan:        make(chan struct{}),
		ctx:             ctx,
		cancel:          cancel,
		circuitBreaker:  NewCircuitBreaker(5, 30*time.Second), // 5 failures, 30s reset timeout
		healthCheck:     NewHealthChecker(),
		progressTracker: NewProgressTracker(),
	}

	if config.EnableMetrics {
		registerMetricsOnce()
		bp.metricsEnabled = true
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())

		// Add health check endpoint
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			if bp.healthCheck.IsHealthy() {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"healthy","circuit_breaker":"` + bp.getCircuitBreakerStateString() + `"}`))
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(`{"status":"unhealthy","circuit_breaker":"` + bp.getCircuitBreakerStateString() + `"}`))
			}
		})

		server := &http.Server{Addr: config.MetricsAddr, Handler: mux}
		bp.metricsServer = server
		go func() {
			logger.Info("Starting metrics server", zap.String("addr", config.MetricsAddr))
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("Metrics server error", zap.Error(err))
			}
		}()
	}

	return bp
}

// getCircuitBreakerStateString returns the circuit breaker state as a string
func (bp *BatchProcessor) getCircuitBreakerStateString() string {
	switch bp.circuitBreaker.GetState() {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Start begins the batch processing
func (bp *BatchProcessor) Start() {
	config := bp.GetConfig()
	bp.logger.Info("Starting batch processor",
		zap.Int("startCount", config.StartCount),
		zap.Int("maxCount", config.MaxCount),
		zap.Int("increment", config.Increment),
	)

	bp.wg.Add(1)
	go bp.processLoop()
}

// Stop gracefully stops the batch processor
func (bp *BatchProcessor) Stop() {
	bp.logger.Info("Stopping batch processor")
	close(bp.stopChan)
	bp.cancel()
	if bp.metricsServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = bp.metricsServer.Shutdown(ctx)
	}
	bp.wg.Wait()
	bp.logger.Info("Batch processor stopped")
}

// processLoop is the main processing loop
func (bp *BatchProcessor) processLoop() {
	defer bp.wg.Done()

	restartCount := 0

	// Outer loop for continuous processing
	for {
		config := bp.GetConfig()
		currentCount := config.StartCount

		// Log cycle start
		if config.ContinuousMode && restartCount > 0 {
			bp.logger.Info("Starting new processing cycle",
				zap.Int("cycleNumber", restartCount+1),
				zap.Int("startCount", config.StartCount),
				zap.Int("maxCount", config.MaxCount))
		}

		// Try to resume from previous progress if enabled
		progressFile := "./logs/batch_progress.json"
		if config.ResumeFromProgress && restartCount == 0 {
			if err := bp.progressTracker.LoadProgress(progressFile); err != nil {
				bp.logger.Warn("Could not load previous progress, starting fresh", zap.Error(err))
			} else {
				lastCount, lastProcessed, totalProcessed, totalSuccessful, totalFailed, startTime := bp.progressTracker.GetProgress()
				if lastCount > config.StartCount {
					currentCount = lastCount + config.Increment
					bp.logger.Info("Resuming from previous progress",
						zap.Int("resumeFromCount", currentCount),
						zap.Time("lastProcessed", lastProcessed),
						zap.Int("totalProcessed", totalProcessed),
						zap.Int("totalSuccessful", totalSuccessful),
						zap.Int("totalFailed", totalFailed),
						zap.Time("originalStartTime", startTime))
				}
			}
		}

		// Setup progress saving ticker if configured
		var progressTicker *time.Ticker
		var progressTickerStop chan struct{}
		if config.progressSaveInterval > 0 {
			progressTicker = time.NewTicker(config.progressSaveInterval)
			progressTickerStop = make(chan struct{})

			go func() {
				for {
					select {
					case <-progressTicker.C:
						if err := bp.progressTracker.SaveProgress(progressFile); err != nil {
							bp.logger.Error("Failed to save progress", zap.Error(err))
						} else {
							currentCount, lastProcessed, totalProcessed, totalSuccessful, totalFailed, _ := bp.progressTracker.GetProgress()
							bp.logger.Debug("Progress saved",
								zap.Int("currentCount", currentCount),
								zap.Time("lastProcessed", lastProcessed),
								zap.Int("totalProcessed", totalProcessed),
								zap.Int("totalSuccessful", totalSuccessful),
								zap.Int("totalFailed", totalFailed))
						}
					case <-progressTickerStop:
						return
					}
				}
			}()

			defer func() {
				close(progressTickerStop)
				progressTicker.Stop()
				// Save final progress
				if err := bp.progressTracker.SaveProgress(progressFile); err != nil {
					bp.logger.Error("Failed to save final progress", zap.Error(err))
				}
			}()
		}

		for currentCount <= config.MaxCount {
			select {
			case <-bp.stopChan:
				bp.logger.Info("Processing loop interrupted")
				return
			default:
				// Pause if inside a configured pause window
				if sleepDur, inPause := bp.sleepDurationIfPaused(time.Now()); inPause {
					bp.logger.Info("In pause window, sleeping until resume", zap.Duration("sleep", sleepDur))
					metricPaused.Set(1)
					select {
					case <-bp.stopChan:
						return
					case <-time.After(sleepDur):
					}
				}
				metricPaused.Set(0)

				// Process current batch
				metricCurrentCount.Set(float64(currentCount))

				// Calculate progress statistics
				totalBatches := (config.MaxCount-config.StartCount)/config.Increment + 1
				completedBatches := (currentCount - config.StartCount) / config.Increment
				progressPercent := float64(completedBatches) / float64(totalBatches) * 100

				bp.logger.Info("Starting batch processing",
					zap.Int("currentCount", currentCount),
					zap.Int("maxCount", config.MaxCount),
					zap.Int("increment", config.Increment),
					zap.Int("completedBatches", completedBatches),
					zap.Int("totalBatches", totalBatches),
					zap.Float64("progressPercent", progressPercent))

				bp.processBatch(currentCount)

				// Increment count
				currentCount += config.Increment

				// Log progress with enhanced statistics
				remainingBatches := (config.MaxCount-currentCount)/config.Increment + 1
				if remainingBatches > 0 {
					_, _, totalProcessed, totalSuccessful, totalFailed, startTime := bp.progressTracker.GetProgress()
					elapsed := time.Since(startTime)
					avgBatchTime := elapsed / time.Duration(totalProcessed)
					estimatedRemaining := time.Duration(remainingBatches) * avgBatchTime

					bp.logger.Info("Batch completed, progress update",
						zap.Int("completedCount", currentCount-config.Increment),
						zap.Int("nextCount", currentCount),
						zap.Int("remainingBatches", remainingBatches),
						zap.Int("totalProcessed", totalProcessed),
						zap.Int("totalSuccessful", totalSuccessful),
						zap.Int("totalFailed", totalFailed),
						zap.Duration("elapsed", elapsed),
						zap.Duration("avgBatchTime", avgBatchTime),
						zap.Duration("estimatedRemaining", estimatedRemaining),
						zap.Float64("progressPercent", progressPercent))
				}

				// Wait before next call if not the last iteration
				if currentCount <= config.MaxCount {
					bp.logger.Info("Waiting before next batch",
						zap.Duration("wait", config.GetWaitDuration()),
					)

					select {
					case <-bp.stopChan:
						return
					case <-time.After(config.GetWaitDuration()):
						// Continue to next iteration
					}
				}
			}
		}

		bp.logger.Info("Batch processing completed",
			zap.Int("cycleNumber", restartCount+1))

		// Check if continuous mode is enabled
		if !config.ContinuousMode {
			bp.logger.Info("Continuous mode disabled, exiting")
			return
		}

		// Check max restarts limit
		if config.MaxRestarts > 0 && restartCount >= config.MaxRestarts {
			bp.logger.Info("Maximum restarts reached, exiting",
				zap.Int("maxRestarts", config.MaxRestarts),
				zap.Int("completedCycles", restartCount+1))
			return
		}

		restartCount++

		// Wait before restarting if configured
		if config.restartDelay > 0 {
			bp.logger.Info("Waiting before restarting processing",
				zap.Duration("restartDelay", config.restartDelay),
				zap.Int("nextCycleNumber", restartCount+1))

			select {
			case <-bp.stopChan:
				bp.logger.Info("Restart cancelled due to stop signal")
				return
			case <-time.After(config.restartDelay):
				// Continue to restart
			}
		}

		// Reset progress tracker for new cycle
		bp.progressTracker.Reset()

		bp.logger.Info("Restarting batch processing from initial count",
			zap.Int("startCount", config.StartCount),
			zap.Int("cycleNumber", restartCount+1))
	}
}

// processBatch processes a single batch with the given count
func (bp *BatchProcessor) processBatch(count int) {
	startTime := time.Now()

	// Check circuit breaker before processing
	if !bp.circuitBreaker.CanExecute() {
		bp.logger.Warn("Circuit breaker is open, skipping batch",
			zap.Int("count", count),
			zap.String("circuitBreakerState", bp.getCircuitBreakerStateString()))

		bp.healthCheck.SetUnhealthy()

		// Update progress tracker for circuit breaker failure
		bp.progressTracker.UpdateProgress(count, 0, count)

		// Save failed result due to circuit breaker
		duration := time.Since(startTime)
		errorDetails := map[string]interface{}{
			"error":   "Circuit breaker is open",
			"retries": 0,
		}
		failedResponse := &BatchOptinResponse{
			Total:        count,
			Successful:   0,
			Failed:       count,
			ErrorDetails: &errorDetails,
		}
		bp.saveResults(count, failedResponse, duration)
		return
	}

	// Get the next entry channel for this batch
	config := bp.GetConfig()
	currentChannel := config.GetNextEntryChannel()

	bp.logger.Info("Processing batch",
		zap.Int("count", count),
		zap.String("telco", config.Telco),
		zap.Strings("productIds", config.ProductIds),
		zap.String("entryChannel", currentChannel),
		zap.String("circuitBreakerState", bp.getCircuitBreakerStateString()),
	)

	// Prepare request
	request := BatchOptinRequest{
		Count:        count,
		EntryChannel: currentChannel,
		ProductIds:   config.ProductIds,
		Telco:        config.Telco,
	}

	// Marshal request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		bp.logger.Error("Failed to marshal request",
			zap.Error(err),
			zap.Int("count", count),
		)
		bp.circuitBreaker.RecordFailure()
		bp.healthCheck.SetUnhealthy()
		return
	}

	// Retry logic with exponential backoff
	var response *BatchOptinResponse
	var lastErr error
	var retry int
	baseDelay := config.GetRetryDuration()

	for retry = 0; retry <= config.MaxRetries; retry++ {
		if retry > 0 {
			// Exponential backoff: delay = baseDelay * 2^(retry-1)
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(retry-1)))
			// Cap the delay at 5 minutes
			if delay > 5*time.Minute {
				delay = 5 * time.Minute
			}

			bp.logger.Info("Retrying batch with exponential backoff",
				zap.Int("retry", retry),
				zap.Int("count", count),
				zap.Duration("delay", delay))

			select {
			case <-bp.stopChan:
				bp.logger.Info("Retry interrupted by stop signal")
				return
			case <-time.After(delay):
				// Continue with retry
			}
		}

		response, lastErr = bp.enqueueAndPollBatch(requestBody)
		if lastErr == nil {
			bp.circuitBreaker.RecordSuccess()
			bp.healthCheck.SetHealthy()
			break
		}

		bp.logger.Warn("Batch attempt failed",
			zap.Int("retry", retry),
			zap.Int("count", count),
			zap.Error(lastErr))
	}

	// After retry loop
	duration := time.Since(startTime)
	if lastErr != nil {
		bp.logger.Error("Batch processing failed after all retries",
			zap.Int("count", count),
			zap.Int("retries", retry),
			zap.Duration("totalDuration", duration),
			zap.Error(lastErr))

		bp.circuitBreaker.RecordFailure()
		bp.healthCheck.SetUnhealthy()

		metricBatchesTotal.WithLabelValues("failed").Inc()
		metricBatchDuration.WithLabelValues("failed").Observe(duration.Seconds())

		// Update progress tracker for failed batch
		bp.progressTracker.UpdateProgress(count, 0, count)

		// Save failed results to file for tracking
		errorDetails := map[string]interface{}{
			"error":    lastErr.Error(),
			"retries":  retry,
			"duration": duration.String(),
		}
		failedResponse := &BatchOptinResponse{
			Total:        count,
			Successful:   0,
			Failed:       count,
			ErrorDetails: &errorDetails,
		}
		bp.saveResults(count, failedResponse, duration)
	} else {
		bp.logger.Info("Batch processing completed successfully",
			zap.Int("count", count),
			zap.Int("successful", response.Successful),
			zap.Int("failed", response.Failed),
			zap.Duration("duration", duration),
			zap.String("circuitBreakerState", bp.getCircuitBreakerStateString()))

		metricBatchesTotal.WithLabelValues("success").Inc()
		metricBatchDuration.WithLabelValues("success").Observe(duration.Seconds())

		// Update progress tracker
		bp.progressTracker.UpdateProgress(count, response.Successful, response.Failed)

		bp.saveResults(count, response, duration)
	}

	// Record retries used (attempts includes the first try)
	if retry > 1 {
		metricRetriesTotal.Add(float64(retry - 1))
	}
}

// enqueueAndPollBatch posts the batch request, receives a jobId, and polls until completion
func (bp *BatchProcessor) enqueueAndPollBatch(requestBody []byte) (*BatchOptinResponse, error) {
	config := bp.GetConfig()
	enqueueURL := fmt.Sprintf("%s/api/v1/subscription-external/batch", config.BaseURL)

	req, err := http.NewRequestWithContext(bp.ctx, "POST", enqueueURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	bp.logger.Info("Sending batch request",
		zap.String("url", enqueueURL),
		zap.String("method", "POST"),
		zap.Int("bodySize", len(requestBody)))

	resp, err := bp.httpClient.Do(req)
	if err != nil {
		bp.logger.Error("HTTP request failed",
			zap.String("url", enqueueURL),
			zap.Error(err))
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	bp.logger.Info("Received HTTP response",
		zap.String("url", enqueueURL),
		zap.Int("statusCode", resp.StatusCode),
		zap.String("status", resp.Status))

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	bp.logger.Debug("Response body",
		zap.String("body", string(body)),
		zap.Int("bodySize", len(body)))

	if resp.StatusCode != http.StatusAccepted { // 202
		return nil, fmt.Errorf("unexpected status code on enqueue: %d, body: %s", resp.StatusCode, string(body))
	}

	var enqueueResp batchJobEnqueueResponse
	if err := json.Unmarshal(body, &enqueueResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal enqueue response: %w", err)
	}
	if enqueueResp.JobID == "" {
		return nil, fmt.Errorf("enqueue response missing jobId")
	}

	bp.logger.Info("Starting job polling",
		zap.String("jobId", enqueueResp.JobID),
		zap.Duration("pollInterval", config.pollDuration))

	// Poll status until terminal state or timeout
	statusURL := fmt.Sprintf("%s/api/v1/subscription-external/batch?jobId=%s", config.BaseURL, enqueueResp.JobID)

	bp.logger.Info("Job polling details",
		zap.String("jobId", enqueueResp.JobID),
		zap.String("statusURL", statusURL),
		zap.Duration("pollInterval", config.pollDuration))

	// Build polling context with optional timeout
	pollCtx := bp.ctx
	var cancel context.CancelFunc
	if config.maxPollDur > 0 {
		pollCtx, cancel = context.WithTimeout(bp.ctx, config.maxPollDur)
		defer cancel()
		bp.logger.Info("Polling with timeout", zap.Duration("timeout", config.maxPollDur))
	}

	ticker := time.NewTicker(config.pollDuration)
	defer ticker.Stop()

	pollCount := 0
	for {
		pollCount++
		select {
		case <-bp.stopChan:
			bp.logger.Info("Polling stopped by stop signal")
			return nil, fmt.Errorf("stopped")
		case <-pollCtx.Done():
			bp.logger.Error("Polling timed out", zap.Duration("timeout", config.maxPollDur))
			return nil, fmt.Errorf("polling timed out after %s", config.maxPollDur)
		case <-ticker.C:
			bp.logger.Debug("Polling job status",
				zap.String("jobId", enqueueResp.JobID),
				zap.Int("pollCount", pollCount))

			statusReq, err := http.NewRequestWithContext(bp.ctx, "GET", statusURL, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create status request: %w", err)
			}
			statusResp, err := bp.httpClient.Do(statusReq)
			if err != nil {
				bp.logger.Error("Status request failed",
					zap.String("jobId", enqueueResp.JobID),
					zap.Int("pollCount", pollCount),
					zap.Error(err))
				return nil, fmt.Errorf("failed to execute status request: %w", err)
			}
			statusBody, err := io.ReadAll(statusResp.Body)
			statusResp.Body.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to read status body: %w", err)
			}
			if statusResp.StatusCode != http.StatusOK {
				bp.logger.Error("Unexpected status code on status request",
					zap.String("jobId", enqueueResp.JobID),
					zap.Int("pollCount", pollCount),
					zap.Int("statusCode", statusResp.StatusCode),
					zap.String("body", string(statusBody)))
				return nil, fmt.Errorf("unexpected status code on status: %d, body: %s", statusResp.StatusCode, string(statusBody))
			}

			var st batchJobStatus
			if err := json.Unmarshal(statusBody, &st); err != nil {
				return nil, fmt.Errorf("failed to unmarshal status: %w", err)
			}

			// Log every 10th poll or when state changes to avoid spam
			if pollCount%10 == 0 || pollCount == 1 {
				bp.logger.Info("Job status update",
					zap.String("jobId", st.ID),
					zap.String("state", string(st.State)),
					zap.Int("total", st.Total),
					zap.Int64("processed", st.Processed),
					zap.Int64("successful", st.Successful),
					zap.Int64("failed", st.Failed),
					zap.Int("pollCount", pollCount),
					zap.Duration("elapsed", time.Since(st.StartedAt)))

				// Warn if job seems stuck (not progressing for more than 5 minutes)
				elapsed := time.Since(st.StartedAt)
				if elapsed > 5*time.Minute && st.Processed == 0 {
					bp.logger.Warn("Job appears to be stuck - no progress for over 5 minutes",
						zap.String("jobId", st.ID),
						zap.Duration("elapsed", elapsed),
						zap.Int64("processed", st.Processed))
				}

				// Warn if job is processing items but not updating success/failure counts
				if st.Processed > 0 && st.Successful == 0 && st.Failed == 0 && elapsed > 2*time.Minute {
					bp.logger.Warn("Job appears to have state management issue - processing items but not updating success/failure counts",
						zap.String("jobId", st.ID),
						zap.Duration("elapsed", elapsed),
						zap.Int64("processed", st.Processed),
						zap.Int64("successful", st.Successful),
						zap.Int64("failed", st.Failed))
				}
			} else {
				bp.logger.Debug("Polled job status",
					zap.String("jobId", st.ID),
					zap.String("state", string(st.State)),
					zap.Int("total", st.Total),
					zap.Int64("processed", st.Processed),
					zap.Int64("successful", st.Successful),
					zap.Int64("failed", st.Failed),
				)
			}

			// Check for job completion or failure
			if st.State == jobStateCompleted || st.State == jobStateFailed {
				bp.logger.Info("Job completed",
					zap.String("jobId", st.ID),
					zap.String("state", string(st.State)),
					zap.Int("total", st.Total),
					zap.Int64("processed", st.Processed),
					zap.Int64("successful", st.Successful),
					zap.Int64("failed", st.Failed),
					zap.Duration("totalTime", time.Since(st.StartedAt)))
				return &BatchOptinResponse{
					Total:      st.Total,
					Successful: int(st.Successful),
					Failed:     int(st.Failed),
				}, nil
			}

			// Check for stuck job with state management issues
			elapsed := time.Since(st.StartedAt)
			if st.Processed > 0 && st.Successful == 0 && st.Failed == 0 && elapsed > 10*time.Minute {
				bp.logger.Error("Job timeout due to state management issues - processing items but not updating success/failure counts",
					zap.String("jobId", st.ID),
					zap.Duration("elapsed", elapsed),
					zap.Int64("processed", st.Processed),
					zap.Int64("successful", st.Successful),
					zap.Int64("failed", st.Failed))
				return nil, fmt.Errorf("job %s timed out after %v - processed %d items but no success/failure counts updated",
					st.ID, elapsed, st.Processed)
			}
		}
	}
}

// saveResults saves the batch processing results to a file
func (bp *BatchProcessor) saveResults(count int, response *BatchOptinResponse, duration time.Duration) {
	config := bp.GetConfig()
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("./logs/batch_results_%d_%s.json", count, timestamp)

	result := map[string]interface{}{
		"timestamp":    time.Now().Format(time.RFC3339),
		"count":        count,
		"telco":        config.Telco,
		"productIds":   config.ProductIds,
		"entryChannel": config.EntryChannel,
		"duration":     duration.String(),
		"response":     response,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		bp.logger.Error("Failed to marshal results",
			zap.Error(err),
			zap.String("filename", filename),
		)
		return
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		bp.logger.Error("Failed to save results",
			zap.Error(err),
			zap.String("filename", filename),
		)
	} else {
		bp.logger.Info("Results saved",
			zap.String("filename", filename),
		)
	}
}

// initLogger initializes the zap logger
func initLogger(debug bool) (*zap.Logger, error) {
	config := zap.NewProductionConfig()

	if debug {
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		config.Development = true
		// Use console encoder for better readability in debug mode
		config.Encoding = "console"
	} else {
		// Use structured JSON logging for production
		config.Encoding = "json"
	}

	// Improve timestamp format for better readability
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Add caller information for better debugging
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	// Ensure logs directory exists
	if err := os.MkdirAll("./logs", 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	config.OutputPaths = []string{"stdout", "./logs/batch_processor.log"}
	config.ErrorOutputPaths = []string{"stderr", "./logs/batch_processor_errors.log"}

	logger, err := config.Build(zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	if err != nil {
		return nil, err
	}

	return logger, nil
}

func parseTimeHMToMinutes(s string) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid time format: %s", s)
	}
	h, err := time.Parse("15", parts[0])
	if err != nil {
		// Fallback simple parse
		var hour int
		_, herr := fmt.Sscanf(parts[0], "%d", &hour)
		if herr != nil {
			return 0, fmt.Errorf("invalid hour: %s", s)
		}
		var min int
		_, merr := fmt.Sscanf(parts[1], "%d", &min)
		if merr != nil {
			return 0, fmt.Errorf("invalid minute: %s", s)
		}
		return hour*60 + min, nil
	}
	m, err := time.Parse("04", parts[1])
	if err != nil {
		var min int
		_, merr := fmt.Sscanf(parts[1], "%d", &min)
		if merr != nil {
			return 0, fmt.Errorf("invalid minute: %s", s)
		}
		var hour int
		_, herr := fmt.Sscanf(parts[0], "%d", &hour)
		if herr != nil {
			return 0, fmt.Errorf("invalid hour: %s", s)
		}
		return hour*60 + min, nil
	}
	// If both parsed, get hours and minutes by formatting
	hour, _ := fmt.Sscanf(parts[0], "%d", new(int))
	_ = hour
	return int(h.Hour())*60 + int(m.Minute()), nil
}

func parsePauseWindows(raw []PauseWindow) ([]parsedPauseWindow, error) {
	var parsed []parsedPauseWindow
	for _, w := range raw {
		start, err := parseTimeHMToMinutes(w.Start)
		if err != nil {
			return nil, err
		}
		end, err := parseTimeHMToMinutes(w.End)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, parsedPauseWindow{startMinutes: start, endMinutes: end})
	}
	return parsed, nil
}

// sleepDurationIfPaused returns the remaining sleep duration and whether now is inside a pause window
func (bp *BatchProcessor) sleepDurationIfPaused(now time.Time) (time.Duration, bool) {
	config := bp.GetConfig()
	if len(config.parsedPauseWindows) == 0 {
		return 0, false
	}
	loc := config.location
	localNow := now.In(loc)
	minutes := localNow.Hour()*60 + localNow.Minute()
	for _, w := range config.parsedPauseWindows {
		if w.startMinutes <= w.endMinutes {
			// Normal window, e.g., 12:00-14:00
			if minutes >= w.startMinutes && minutes < w.endMinutes {
				// sleep until end
				endToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), w.endMinutes/60, w.endMinutes%60, 0, 0, loc)
				return endToday.Sub(localNow), true
			}
		} else {
			// Wraps midnight, e.g., 22:00-06:00
			if minutes >= w.startMinutes || minutes < w.endMinutes {
				// compute end: if before endMinutes, today at end; if after start, next day end
				endDay := localNow
				if minutes >= w.startMinutes {
					endDay = endDay.Add(24 * time.Hour)
				}
				end := time.Date(endDay.Year(), endDay.Month(), endDay.Day(), w.endMinutes/60, w.endMinutes%60, 0, 0, loc)
				return end.Sub(localNow), true
			}
		}
	}
	return 0, false
}

// loadConfig loads configuration from file or uses defaults
func loadConfig(configFile string) (*ProcessorConfig, error) {
	// Default configuration
	config := &ProcessorConfig{
		BaseURL:          "http://localhost:8083",
		StartCount:       1000,
		MaxCount:         5000000,
		Increment:        1000,
		Telco:            "AirtelTigo",
		EntryChannel:     "USSD",
		EntryChannels:    []string{"USSD"}, // Default to single channel
		ProductIds:       []string{"8509"},
		WaitBetweenCalls: "30s",
		MaxRetries:       3,
		RetryDelay:       "5s",
		PollInterval:     "2s",
		EnableMetrics:    true,
		MetricsAddr:      ":9101",
	}

	// If config file is specified, load it
	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Handle entry channels configuration
	// If EntryChannels is not set but EntryChannel is, use EntryChannel as the only channel
	if len(config.EntryChannels) == 0 && config.EntryChannel != "" {
		config.EntryChannels = []string{config.EntryChannel}
	}
	// If EntryChannels is set but EntryChannel is not, set EntryChannel to the first channel for backward compatibility
	if len(config.EntryChannels) > 0 && config.EntryChannel == "" {
		config.EntryChannel = config.EntryChannels[0]
	}
	// If neither is set, use default
	if len(config.EntryChannels) == 0 && config.EntryChannel == "" {
		config.EntryChannels = []string{"USSD"}
		config.EntryChannel = "USSD"
	}
	// Ensure EntryChannels is always set based on EntryChannel if it's not already set
	if len(config.EntryChannels) == 0 {
		config.EntryChannels = []string{config.EntryChannel}
	}

	// Parse duration strings
	var err error
	config.waitDuration, err = time.ParseDuration(config.WaitBetweenCalls)
	if err != nil {
		return nil, fmt.Errorf("invalid wait_between_calls duration: %w", err)
	}

	config.retryDuration, err = time.ParseDuration(config.RetryDelay)
	if err != nil {
		return nil, fmt.Errorf("invalid retry_delay duration: %w", err)
	}

	// Poll interval
	if config.PollInterval == "" {
		config.PollInterval = "2s"
	}
	config.pollDuration, err = time.ParseDuration(config.PollInterval)
	if err != nil {
		return nil, fmt.Errorf("invalid poll_interval duration: %w", err)
	}

	// Max polling duration (optional)
	if strings.TrimSpace(config.MaxPollingDuration) != "" {
		config.maxPollDur, err = time.ParseDuration(config.MaxPollingDuration)
		if err != nil {
			return nil, fmt.Errorf("invalid max_polling_duration: %w", err)
		}
	}

	// Progress tracking
	if strings.TrimSpace(config.SaveProgressInterval) != "" {
		config.progressSaveInterval, err = time.ParseDuration(config.SaveProgressInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid save_progress_interval duration: %w", err)
		}
	}

	// Restart delay for continuous processing
	if strings.TrimSpace(config.RestartDelay) != "" {
		config.restartDelay, err = time.ParseDuration(config.RestartDelay)
		if err != nil {
			return nil, fmt.Errorf("invalid restart_delay duration: %w", err)
		}
	}

	// Timezone
	if strings.TrimSpace(config.Timezone) != "" {
		loc, tzErr := time.LoadLocation(config.Timezone)
		if tzErr != nil {
			return nil, fmt.Errorf("invalid timezone: %w", tzErr)
		}
		config.location = loc
	} else {
		config.location = time.Local
	}

	// Pause windows
	if len(config.PauseWindows) > 0 {
		parsed, pwErr := parsePauseWindows(config.PauseWindows)
		if pwErr != nil {
			return nil, fmt.Errorf("invalid pause_windows: %w", pwErr)
		}
		config.parsedPauseWindows = parsed
	}

	return config, nil
}

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

// CircuitBreaker implements a simple circuit breaker pattern
type CircuitBreaker struct {
	maxFailures     int
	resetTimeout    time.Duration
	failureCount    int
	lastFailureTime time.Time
	state           CircuitBreakerState
	mutex           sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        CircuitClosed,
	}
}

// CanExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			cb.mutex.RUnlock()
			cb.mutex.Lock()
			cb.state = CircuitHalfOpen
			cb.mutex.Unlock()
			cb.mutex.RLock()
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful execution
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failureCount = 0
	cb.state = CircuitClosed
}

// RecordFailure records a failed execution
func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()

	if cb.failureCount >= cb.maxFailures {
		cb.state = CircuitOpen
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// HealthChecker checks the health of the service
type HealthChecker struct {
	healthy bool
	mutex   sync.RWMutex
}

// NewHealthChecker creates a new health checker
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		healthy: true,
	}
}

// SetHealthy sets the health to healthy
func (hc *HealthChecker) SetHealthy() {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	hc.healthy = true
}

// SetUnhealthy sets the health to unhealthy
func (hc *HealthChecker) SetUnhealthy() {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	hc.healthy = false
}

// IsHealthy returns the current health status
func (hc *HealthChecker) IsHealthy() bool {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()
	return hc.healthy
}

// ProgressTracker tracks processing progress
type ProgressTracker struct {
	CurrentCount    int       `json:"current_count"`
	LastProcessed   time.Time `json:"last_processed"`
	TotalProcessed  int       `json:"total_processed"`
	TotalSuccessful int       `json:"total_successful"`
	TotalFailed     int       `json:"total_failed"`
	StartTime       time.Time `json:"start_time"`
	mutex           sync.RWMutex
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		StartTime: time.Now(),
	}
}

// UpdateProgress updates the progress with the latest batch results
func (pt *ProgressTracker) UpdateProgress(count int, successful, failed int) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	pt.CurrentCount = count
	pt.LastProcessed = time.Now()
	pt.TotalProcessed++
	pt.TotalSuccessful += successful
	pt.TotalFailed += failed
}

// GetProgress returns the current progress
func (pt *ProgressTracker) GetProgress() (int, time.Time, int, int, int, time.Time) {
	pt.mutex.RLock()
	defer pt.mutex.RUnlock()

	return pt.CurrentCount, pt.LastProcessed, pt.TotalProcessed, pt.TotalSuccessful, pt.TotalFailed, pt.StartTime
}

// SaveProgress saves progress to file
func (pt *ProgressTracker) SaveProgress(filename string) error {
	pt.mutex.RLock()
	defer pt.mutex.RUnlock()

	data, err := json.MarshalIndent(pt, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal progress: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

// LoadProgress loads progress from file
func (pt *ProgressTracker) LoadProgress(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read progress file: %w", err)
	}

	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	return json.Unmarshal(data, pt)
}

// Reset resets the progress tracker for a new processing cycle
func (pt *ProgressTracker) Reset() {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	pt.CurrentCount = 0
	pt.LastProcessed = time.Time{}
	pt.TotalProcessed = 0
	pt.TotalSuccessful = 0
	pt.TotalFailed = 0
	pt.StartTime = time.Now()
}

func main() {
	// Parse command line flags
	var (
		configFile    = flag.String("config", "", "Path to configuration file")
		baseURL       = flag.String("url", "http://localhost:8080", "Base URL of the subscription service")
		startCount    = flag.Int("start", 1000, "Starting count value")
		maxCount      = flag.Int("max", 5000000, "Maximum count value")
		increment     = flag.Int("increment", 1000, "Increment value")
		telco         = flag.String("telco", "AirtelTigo", "Telco name")
		entryChannel  = flag.String("channel", "USSD", "Entry channel")
		entryChannels = flag.String("channels", "", "Comma-separated entry channels for rotation (e.g., USSD,WEB,SMS)")
		productIds    = flag.String("products", "8509", "Comma-separated product IDs")
		waitTime      = flag.Duration("wait", 30*time.Second, "Wait time between calls")
		debug         = flag.Bool("debug", false, "Enable debug logging")
		runOnce       = flag.Bool("once", false, "Run only once for the start count value")
		dryRun        = flag.Bool("dry-run", false, "Dry run mode - only log what would be done")
		pollInterval  = flag.Duration("poll", 2*time.Second, "Polling interval for job status")
		enableMetrics = flag.Bool("metrics", true, "Enable Prometheus metrics endpoint")
		metricsAddr   = flag.String("metrics-addr", ":9101", "Address for metrics endpoint (host:port or :port)")
		timezone      = flag.String("timezone", "", "IANA timezone for pause windows (e.g. Africa/Accra). Empty=local")
		maxPoll       = flag.Duration("max-poll", 0, "Optional maximum duration to poll a job before failing (0=disabled)")
		pauseWindows  = flag.String("pause-windows", "", "Optional semicolon-separated pause windows HH:MM-HH:MM;HH:MM-HH:MM")
		hotReload     = flag.Bool("hot-reload", false, "Enable hot reloading of configuration file")
		watchDebug    = flag.Bool("watch-debug", false, "Enable debug logging for config watcher")

		// Continuous processing flags
		continuousMode = flag.Bool("continuous", false, "Enable continuous processing mode (restart after completion)")
		restartDelay   = flag.String("restart-delay", "", "Delay before restarting (e.g., 5m, 1h)")
		maxRestarts    = flag.Int("max-restarts", 0, "Maximum number of restarts (0 = unlimited)")
	)

	flag.Parse()

	// Initialize logger
	logger, err := initLogger(*debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Load configuration
	config, err := loadConfig(*configFile)
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Override config with command line flags if provided
	if *baseURL != "http://localhost:8080" {
		config.BaseURL = *baseURL
	}
	if *startCount != 1000 {
		config.StartCount = *startCount
	}
	if *maxCount != 5000000 {
		config.MaxCount = *maxCount
	}
	if *increment != 1000 {
		config.Increment = *increment
	}
	if *telco != "AirtelTigo" {
		config.Telco = *telco
	}
	// Only override entry channel if explicitly provided via command line
	// Check if the flag value differs from the config file value
	if *entryChannel != config.EntryChannel {
		// Only override if the flag value is not the default value
		// This prevents overriding config file values when no flag was provided
		if *entryChannel != "USSD" {
			config.EntryChannel = *entryChannel
			// Update EntryChannels to use the single channel from command line
			config.EntryChannels = []string{*entryChannel}
		}
	}
	if *entryChannels != "" {
		channels := strings.Split(*entryChannels, ",")
		config.EntryChannels = make([]string, 0, len(channels))
		for _, ch := range channels {
			ch = strings.TrimSpace(ch)
			if ch != "" {
				config.EntryChannels = append(config.EntryChannels, ch)
			}
		}
		// Update EntryChannel to first channel for backward compatibility
		if len(config.EntryChannels) > 0 {
			config.EntryChannel = config.EntryChannels[0]
		}
	}
	if *productIds != "8509" {
		// Parse comma-separated product IDs
		productIdList := []string{}
		for _, id := range bytes.Split([]byte(*productIds), []byte(",")) {
			productIdList = append(productIdList, string(bytes.TrimSpace(id)))
		}
		config.ProductIds = productIdList
	}
	// Only override wait time if explicitly provided via command line
	// Check if the flag value differs from the config file value
	if *waitTime != config.waitDuration {
		// Only override if the flag value is not the default value
		// This prevents overriding config file values when no flag was provided
		if *waitTime != 30*time.Second {
			config.WaitBetweenCalls = waitTime.String()
			config.waitDuration = *waitTime
		}
	}
	// Only override poll interval if explicitly provided via command line
	// Check if the flag value differs from the config file value
	if *pollInterval != config.pollDuration {
		// Only override if the flag value is not the default value
		// This prevents overriding config file values when no flag was provided
		if *pollInterval != 2*time.Second {
			config.PollInterval = pollInterval.String()
			config.pollDuration = *pollInterval
		}
	}
	if *enableMetrics != config.EnableMetrics {
		config.EnableMetrics = *enableMetrics
	}
	if *metricsAddr != "" && *metricsAddr != config.MetricsAddr {
		config.MetricsAddr = *metricsAddr
	}
	if *timezone != "" {
		config.Timezone = *timezone
		loc, tzErr := time.LoadLocation(config.Timezone)
		if tzErr != nil {
			logger.Fatal("Invalid timezone", zap.Error(tzErr))
		}
		config.location = loc
	}
	if *maxPoll > 0 {
		config.maxPollDur = *maxPoll
		config.MaxPollingDuration = maxPoll.String()
	}
	if strings.TrimSpace(*pauseWindows) != "" {
		windows := strings.Split(*pauseWindows, ";")
		pws := make([]PauseWindow, 0, len(windows))
		for _, w := range windows {
			parts := strings.Split(strings.TrimSpace(w), "-")
			if len(parts) != 2 {
				logger.Fatal("Invalid pause window format", zap.String("window", w))
			}
			pws = append(pws, PauseWindow{Start: strings.TrimSpace(parts[0]), End: strings.TrimSpace(parts[1])})
		}
		parsed, pwErr := parsePauseWindows(pws)
		if pwErr != nil {
			logger.Fatal("Invalid pause windows", zap.Error(pwErr))
		}
		config.PauseWindows = pws
		config.parsedPauseWindows = parsed
	}

	// Handle continuous processing configuration
	if *continuousMode {
		config.ContinuousMode = *continuousMode
	}
	if *restartDelay != "" {
		parsed, err := time.ParseDuration(*restartDelay)
		if err != nil {
			logger.Fatal("Invalid restart delay duration", zap.String("restartDelay", *restartDelay), zap.Error(err))
		}
		config.RestartDelay = *restartDelay
		config.restartDelay = parsed
	}
	if *maxRestarts > 0 {
		config.MaxRestarts = *maxRestarts
	}

	// If run once mode, set max count to start count
	if *runOnce {
		config.MaxCount = config.StartCount
	}

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll("./logs", 0755); err != nil {
		logger.Fatal("Failed to create logs directory", zap.Error(err))
	}
	logger.Info("Logs directory ready", zap.String("path", "./logs"))

	// Initialize config watcher if hot reloading is enabled
	var configWatcher *ConfigWatcher
	if *hotReload && *configFile != "" {
		var err error
		configWatcher, err = NewConfigWatcher(*configFile, logger)
		if err != nil {
			logger.Warn("Failed to initialize config watcher, continuing without hot reload", zap.Error(err))
		} else {
			// Set the initial configuration
			configWatcher.currentConfig = config

			// Start the config watcher
			if err := configWatcher.Start(); err != nil {
				logger.Warn("Failed to start config watcher, continuing without hot reload", zap.Error(err))
				configWatcher = nil
			} else {
				logger.Info("Configuration hot reloading enabled", zap.String("configFile", *configFile))

				// If watch debug is enabled, log additional information
				if *watchDebug {
					logger.Info("Config watcher debug mode enabled - will log all file events")
					watchedFiles := configWatcher.ListWatchedFiles()
					logger.Info("Watching files/directories", zap.Strings("watched", watchedFiles))
				}
			}
		}
	}

	// Log configuration
	logger.Info("Configuration loaded",
		zap.String("baseURL", config.BaseURL),
		zap.Int("startCount", config.StartCount),
		zap.Int("maxCount", config.MaxCount),
		zap.Int("increment", config.Increment),
		zap.String("telco", config.Telco),
		zap.String("entryChannel", config.EntryChannel),
		zap.Strings("entryChannels", config.EntryChannels),
		zap.Strings("productIds", config.ProductIds),
		zap.Duration("waitBetweenCalls", config.GetWaitDuration()),
		zap.Duration("pollInterval", config.pollDuration),
		zap.Bool("runOnce", *runOnce),
		zap.Bool("dryRun", *dryRun),
		zap.Bool("metrics", config.EnableMetrics),
		zap.String("metricsAddr", config.MetricsAddr),
		zap.String("timezone", config.Timezone),
		zap.Any("pauseWindows", config.PauseWindows),
		zap.Duration("maxPollingDuration", config.maxPollDur),
		zap.Bool("continuousMode", config.ContinuousMode),
		zap.Duration("restartDelay", config.restartDelay),
		zap.Int("maxRestarts", config.MaxRestarts),
	)

	// If dry run mode, just log what would be done and exit
	if *dryRun {
		logger.Info("DRY RUN MODE - Showing what would be executed")
		currentCount := config.StartCount
		batchNumber := 1

		for currentCount <= config.MaxCount {
			logger.Info("Would process batch",
				zap.Int("batchNumber", batchNumber),
				zap.Int("count", currentCount),
			)
			currentCount += config.Increment
			batchNumber++
		}

		totalBatches := batchNumber - 1
		estimatedTime := time.Duration(totalBatches-1) * config.GetWaitDuration()
		logger.Info("Dry run summary",
			zap.Int("totalBatches", totalBatches),
			zap.Duration("estimatedTime", estimatedTime),
		)
		return
	}

	// Create batch processor
	processor := NewBatchProcessor(config, logger)

	// Start configuration reload handler if hot reloading is enabled
	if configWatcher != nil {
		configStopChan := make(chan struct{})
		go func() {
			for {
				select {
				case newConfig := <-configWatcher.GetReloadChannel():
					logger.Info("Received configuration reload event")
					processor.UpdateConfig(newConfig)
				case <-configStopChan:
					return
				}
			}
		}()

		// Clean up config watcher goroutine on exit
		defer close(configStopChan)
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start processing
	processor.Start()

	// Wait for signal
	sig := <-sigChan
	logger.Info("Received signal", zap.String("signal", sig.String()))

	// Stop config watcher if enabled
	if configWatcher != nil {
		configWatcher.Stop()
	}

	// Stop processor
	processor.Stop()

	logger.Info("Batch processor exited successfully")
}
