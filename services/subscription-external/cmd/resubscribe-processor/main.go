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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ResubscribeRequest represents the request structure for re-subscription
type ResubscribeRequest struct {
	Telco         string   `json:"telco"`
	EntryChannel  string   `json:"entry_channel"`
	EntryChannels []string `json:"entry_channels,omitempty"`
	ProductIds    []string `json:"product_ids"`
	MSISDNS       []string `json:"msisdns,omitempty"`
	StartIndex    int      `json:"start_index,omitempty"`
	EndIndex      int      `json:"end_index,omitempty"`
}

// ResubscribeResponse represents the response structure
type ResubscribeResponse struct {
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

// ProcessorConfig holds the configuration for the resubscribe processor
type ProcessorConfig struct {
	BaseURL          string   `json:"base_url"`
	Telco            string   `json:"telco"`
	EntryChannel     string   `json:"entry_channel"`            // Single channel (legacy support)
	EntryChannels    []string `json:"entry_channels,omitempty"` // Multiple channels for rotation
	ProductIds       []string `json:"product_ids"`
	MSISDNS          []string `json:"msisdns,omitempty"`     // Optional explicit MSISDNs to resubscribe
	StartIndex       int      `json:"start_index,omitempty"` // Window start index
	EndIndex         int      `json:"end_index,omitempty"`   // Window end index
	WaitBetweenCalls string   `json:"wait_between_calls"`    // String for JSON unmarshaling
	MaxRetries       int      `json:"max_retries"`
	RetryDelay       string   `json:"retry_delay"` // String for JSON unmarshaling
	PollInterval     string   `json:"poll_interval"`

	// Metrics
	EnableMetrics bool   `json:"enable_metrics"`
	MetricsAddr   string `json:"metrics_addr"` // e.g. ":9102"

	// Pause/resume schedule
	PauseWindows []PauseWindow `json:"pause_windows,omitempty"`
	Timezone     string        `json:"timezone,omitempty"` // IANA TZ, e.g. "Africa/Accra"; empty means local

	// Safety: max time to poll a single job before failing
	MaxPollingDuration string `json:"max_polling_duration,omitempty"`

	// Parsed durations (not exported)
	waitDuration  time.Duration
	retryDuration time.Duration
	pollDuration  time.Duration
	maxPollDur    time.Duration

	// Derived
	location           *time.Location
	parsedPauseWindows []parsedPauseWindow

	// Entry channel rotation
	currentChannelIndex int
	channelMutex        sync.Mutex
}

// ResubscribeProcessor handles the resubscribe processing logic
type ResubscribeProcessor struct {
	config         *ProcessorConfig
	logger         *zap.Logger
	httpClient     *http.Client
	stopChan       chan struct{}
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
	metricsEnabled bool
	metricsServer  *http.Server
}

// Prometheus metrics
var (
	metricResubscribesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "resubscribe_processor_resubscribes_total",
			Help: "Total number of processed resubscribes by outcome",
		},
		[]string{"outcome"},
	)
	metricResubscribeDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "resubscribe_processor_resubscribe_duration_seconds",
			Help:    "Duration of resubscribe processing in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"outcome"},
	)
	metricRetriesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "resubscribe_processor_retries_total",
			Help: "Total number of retry attempts across all resubscribes",
		},
	)
	metricPaused = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "resubscribe_processor_paused",
			Help: "Whether the processor is currently paused (1) or running (0)",
		},
	)
)

func registerMetricsOnce() {
	// Register may panic if duplicate; use MustRegister guarded by recover in case already registered by multiple runs
	// However in a single process, this will be called once.
	prometheus.MustRegister(metricResubscribesTotal)
	prometheus.MustRegister(metricResubscribeDuration)
	prometheus.MustRegister(metricRetriesTotal)
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

// NewResubscribeProcessor creates a new resubscribe processor instance
func NewResubscribeProcessor(config *ProcessorConfig, logger *zap.Logger) *ResubscribeProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	rp := &ResubscribeProcessor{
		config:     config,
		logger:     logger,
		httpClient: &http.Client{Timeout: 5 * time.Minute}, // Long timeout for large batches
		stopChan:   make(chan struct{}),
		ctx:        ctx,
		cancel:     cancel,
	}

	if config.EnableMetrics {
		registerMetricsOnce()
		rp.metricsEnabled = true
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		server := &http.Server{Addr: config.MetricsAddr, Handler: mux}
		rp.metricsServer = server
		go func() {
			logger.Info("Starting metrics server", zap.String("addr", config.MetricsAddr))
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("Metrics server error", zap.Error(err))
			}
		}()
	}

	return rp
}

// Start begins the resubscribe processing
func (rp *ResubscribeProcessor) Start() {
	rp.logger.Info("Starting resubscribe processor",
		zap.String("telco", rp.config.Telco),
		zap.Strings("productIds", rp.config.ProductIds),
		zap.String("entryChannel", rp.config.EntryChannel),
		zap.Strings("entryChannels", rp.config.EntryChannels),
		zap.Int("msisdnCount", len(rp.config.MSISDNS)),
		zap.Int("startIndex", rp.config.StartIndex),
		zap.Int("endIndex", rp.config.EndIndex),
	)

	rp.wg.Add(1)
	go rp.processLoop()
}

// Stop gracefully stops the resubscribe processor
func (rp *ResubscribeProcessor) Stop() {
	rp.logger.Info("Stopping resubscribe processor")
	close(rp.stopChan)
	rp.cancel()
	if rp.metricsServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = rp.metricsServer.Shutdown(ctx)
	}
	rp.wg.Wait()
	rp.logger.Info("Resubscribe processor stopped")
}

// processLoop is the main processing loop
func (rp *ResubscribeProcessor) processLoop() {
	defer rp.wg.Done()

	for {
		select {
		case <-rp.stopChan:
			rp.logger.Info("Processing loop interrupted")
			return
		default:
			// Pause if inside a configured pause window
			if sleepDur, inPause := rp.sleepDurationIfPaused(time.Now()); inPause {
				rp.logger.Info("In pause window, sleeping until resume", zap.Duration("sleep", sleepDur))
				metricPaused.Set(1)
				select {
				case <-rp.stopChan:
					return
				case <-time.After(sleepDur):
				}
			}
			metricPaused.Set(0)

			// Process resubscribe request
			rp.processResubscribe()

			// Wait before next call
			rp.logger.Info("Waiting before next resubscribe",
				zap.Duration("wait", rp.config.GetWaitDuration()),
			)

			select {
			case <-rp.stopChan:
				return
			case <-time.After(rp.config.GetWaitDuration()):
				// Continue to next iteration
			}
		}
	}
}

// processResubscribe processes a single resubscribe request
func (rp *ResubscribeProcessor) processResubscribe() {
	startTime := time.Now()

	// Get the next entry channel for this request
	currentChannel := rp.config.GetNextEntryChannel()

	rp.logger.Info("Processing resubscribe request",
		zap.String("telco", rp.config.Telco),
		zap.Strings("productIds", rp.config.ProductIds),
		zap.String("entryChannel", currentChannel),
		zap.Int("msisdnCount", len(rp.config.MSISDNS)),
		zap.Int("startIndex", rp.config.StartIndex),
		zap.Int("endIndex", rp.config.EndIndex),
	)

	// Prepare request
	request := ResubscribeRequest{
		Telco:         rp.config.Telco,
		EntryChannel:  currentChannel,
		EntryChannels: rp.config.EntryChannels,
		ProductIds:    rp.config.ProductIds,
		MSISDNS:       rp.config.MSISDNS,
		StartIndex:    rp.config.StartIndex,
		EndIndex:      rp.config.EndIndex,
	}

	// Marshal request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		rp.logger.Error("Failed to marshal request",
			zap.Error(err),
		)
		return
	}

	// Execute request with retries (async job pattern)
	var response *ResubscribeResponse
	var lastErr error
	attempts := 0

	for retry := 0; retry <= rp.config.MaxRetries; retry++ {
		attempts++
		if retry > 0 {
			rp.logger.Info("Retrying request",
				zap.Int("retry", retry),
			)
			time.Sleep(rp.config.GetRetryDuration())
		}

		response, lastErr = rp.enqueueAndPollResubscribe(requestBody)
		if lastErr == nil {
			break
		}

		rp.logger.Warn("Request failed, will retry",
			zap.Error(lastErr),
			zap.Int("retry", retry),
		)
	}

	// Log results
	duration := time.Since(startTime)

	if lastErr != nil {
		rp.logger.Error("Resubscribe processing failed after all retries",
			zap.Error(lastErr),
			zap.Int("retries", rp.config.MaxRetries),
			zap.Duration("duration", duration),
		)
		metricResubscribesTotal.WithLabelValues("failure").Inc()
		metricResubscribeDuration.WithLabelValues("failure").Observe(duration.Seconds())
	} else if response != nil {
		successRate := float64(response.Successful) / float64(response.Total) * 100

		rp.logger.Info("Resubscribe processing completed",
			zap.Int("total", response.Total),
			zap.Int("successful", response.Successful),
			zap.Int("failed", response.Failed),
			zap.Float64("successRate", successRate),
			zap.Duration("duration", duration),
			zap.Any("errorDetails", response.ErrorDetails),
		)

		metricResubscribesTotal.WithLabelValues("success").Inc()
		metricResubscribeDuration.WithLabelValues("success").Observe(duration.Seconds())

		// Save results to file for record keeping
		rp.saveResults(response, duration)
	}

	// Record retries used (attempts includes the first try)
	if attempts > 1 {
		metricRetriesTotal.Add(float64(attempts - 1))
	}
}

// enqueueAndPollResubscribe posts the resubscribe request, receives a jobId, and polls until completion
func (rp *ResubscribeProcessor) enqueueAndPollResubscribe(requestBody []byte) (*ResubscribeResponse, error) {
	enqueueURL := fmt.Sprintf("%s/api/v1/subscription-external/resubscribe", rp.config.BaseURL)

	req, err := http.NewRequestWithContext(rp.ctx, "POST", enqueueURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := rp.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

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

	// Poll status until terminal state or timeout
	statusURL := fmt.Sprintf("%s/api/v1/subscription-external/batch?jobId=%s", rp.config.BaseURL, enqueueResp.JobID)

	// Build polling context with optional timeout
	pollCtx := rp.ctx
	var cancel context.CancelFunc
	if rp.config.maxPollDur > 0 {
		pollCtx, cancel = context.WithTimeout(rp.ctx, rp.config.maxPollDur)
		defer cancel()
	}

	ticker := time.NewTicker(rp.config.pollDuration)
	defer ticker.Stop()

	for {
		select {
		case <-rp.stopChan:
			return nil, fmt.Errorf("stopped")
		case <-pollCtx.Done():
			return nil, fmt.Errorf("polling timed out after %s", rp.config.maxPollDur)
		case <-ticker.C:
			statusReq, err := http.NewRequestWithContext(rp.ctx, "GET", statusURL, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create status request: %w", err)
			}
			statusResp, err := rp.httpClient.Do(statusReq)
			if err != nil {
				return nil, fmt.Errorf("failed to execute status request: %w", err)
			}
			statusBody, err := io.ReadAll(statusResp.Body)
			statusResp.Body.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to read status body: %w", err)
			}
			if statusResp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("unexpected status code on status: %d, body: %s", statusResp.StatusCode, string(statusBody))
			}

			var st batchJobStatus
			if err := json.Unmarshal(statusBody, &st); err != nil {
				return nil, fmt.Errorf("failed to unmarshal status: %w", err)
			}

			rp.logger.Debug("Polled job status",
				zap.String("jobId", st.ID),
				zap.String("state", string(st.State)),
				zap.Int("total", st.Total),
				zap.Int64("processed", st.Processed),
				zap.Int64("successful", st.Successful),
				zap.Int64("failed", st.Failed),
			)

			if st.State == jobStateCompleted || st.State == jobStateFailed {
				// Normalize to ResubscribeResponse for downstream logging/saving
				resp := &ResubscribeResponse{
					Total:      st.Total,
					Successful: int(st.Successful),
					Failed:     int(st.Failed),
				}
				if st.ErrorDetails != nil {
					resp.ErrorDetails = &st.ErrorDetails
				}
				return resp, nil
			}
		}
	}
}

// saveResults saves the resubscribe processing results to a file
func (rp *ResubscribeProcessor) saveResults(response *ResubscribeResponse, duration time.Duration) {
	// Ensure logs directory exists
	if err := os.MkdirAll("./logs", 0755); err != nil {
		rp.logger.Error("Failed to create logs directory",
			zap.Error(err),
		)
		return
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("./logs/resubscribe_results_%s.json", timestamp)

	result := map[string]interface{}{
		"timestamp":     time.Now().Format(time.RFC3339),
		"telco":         rp.config.Telco,
		"productIds":    rp.config.ProductIds,
		"entryChannel":  rp.config.EntryChannel,
		"entryChannels": rp.config.EntryChannels,
		"msisdnCount":   len(rp.config.MSISDNS),
		"startIndex":    rp.config.StartIndex,
		"endIndex":      rp.config.EndIndex,
		"duration":      duration.String(),
		"response":      response,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		rp.logger.Error("Failed to marshal results",
			zap.Error(err),
			zap.String("filename", filename),
		)
		return
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		rp.logger.Error("Failed to save results",
			zap.Error(err),
			zap.String("filename", filename),
		)
	} else {
		rp.logger.Info("Results saved",
			zap.String("filename", filename),
		)
	}
}

// initLogger initializes the zap logger
func initLogger(debug bool) (*zap.Logger, error) {
	// Create logs directory if it doesn't exist
	if err := os.MkdirAll("./logs", 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	config := zap.NewProductionConfig()

	if debug {
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		config.Development = true
	}

	config.OutputPaths = []string{"stdout", "./logs/resubscribe_processor.log"}
	config.ErrorOutputPaths = []string{"stderr"}

	logger, err := config.Build()
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
func (rp *ResubscribeProcessor) sleepDurationIfPaused(now time.Time) (time.Duration, bool) {
	if len(rp.config.parsedPauseWindows) == 0 {
		return 0, false
	}
	loc := rp.config.location
	localNow := now.In(loc)
	minutes := localNow.Hour()*60 + localNow.Minute()
	for _, w := range rp.config.parsedPauseWindows {
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
		Telco:            "AirtelTigo",
		EntryChannel:     "USSD",
		EntryChannels:    []string{"USSD"}, // Default to single channel
		ProductIds:       []string{"8509"},
		MSISDNS:          []string{}, // Empty by default, will use windowing
		StartIndex:       0,          // Full range by default
		EndIndex:         -1,         // Full range by default
		WaitBetweenCalls: "30s",
		MaxRetries:       3,
		RetryDelay:       "5s",
		PollInterval:     "2s",
		EnableMetrics:    true,
		MetricsAddr:      ":9102", // Different port from batch-processor
	}

	// If config file is specified, load it
	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file '%s': %w", configFile, err)
		}

		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file '%s': %w", configFile, err)
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

func main() {
	// Parse command line flags
	var (
		configFile    = flag.String("config", "config.json", "Path to configuration file")
		baseURL       = flag.String("url", "http://localhost:8083", "Base URL of the subscription service")
		telco         = flag.String("telco", "AirtelTigo", "Telco name")
		entryChannel  = flag.String("channel", "USSD", "Entry channel")
		entryChannels = flag.String("channels", "", "Comma-separated entry channels for rotation (e.g., USSD,WEB,SMS)")
		productIds    = flag.String("products", "8509", "Comma-separated product IDs")
		msisdns       = flag.String("msisdns", "", "Comma-separated MSISDNs to resubscribe (optional)")
		startIndex    = flag.Int("start-index", 0, "Start index for windowing (0 or -1 for full range)")
		endIndex      = flag.Int("end-index", -1, "End index for windowing (-1 for full range)")
		waitTime      = flag.Duration("wait", 30*time.Second, "Wait time between calls")
		debug         = flag.Bool("debug", false, "Enable debug logging")
		runOnce       = flag.Bool("once", false, "Run only once")
		dryRun        = flag.Bool("dry-run", false, "Dry run mode - only log what would be done")
		pollInterval  = flag.Duration("poll", 2*time.Second, "Polling interval for job status")
		enableMetrics = flag.Bool("metrics", true, "Enable Prometheus metrics endpoint")
		metricsAddr   = flag.String("metrics-addr", ":9102", "Address for metrics endpoint (host:port or :port)")
		timezone      = flag.String("timezone", "", "IANA timezone for pause windows (e.g. Africa/Accra). Empty=local")
		maxPoll       = flag.Duration("max-poll", 0, "Optional maximum duration to poll a job before failing (0=disabled)")
		pauseWindows  = flag.String("pause-windows", "", "Optional semicolon-separated pause windows HH:MM-HH:MM;HH:MM-HH:MM")
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
	if *baseURL != "http://localhost:8083" {
		config.BaseURL = *baseURL
	}
	if *telco != "AirtelTigo" {
		config.Telco = *telco
	}
	if *entryChannel != "USSD" {
		config.EntryChannel = *entryChannel
		// Update EntryChannels to use the single channel from command line
		config.EntryChannels = []string{*entryChannel}
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
	if *msisdns != "" {
		// Parse comma-separated MSISDNs
		msisdnList := []string{}
		for _, msisdn := range bytes.Split([]byte(*msisdns), []byte(",")) {
			msisdn := string(bytes.TrimSpace(msisdn))
			if msisdn != "" {
				msisdnList = append(msisdnList, msisdn)
			}
		}
		config.MSISDNS = msisdnList
	}
	if *startIndex != 0 {
		config.StartIndex = *startIndex
	}
	if *endIndex != -1 {
		config.EndIndex = *endIndex
	}
	if *waitTime != 30*time.Second {
		config.WaitBetweenCalls = waitTime.String()
		config.waitDuration = *waitTime
	}
	if *pollInterval != 2*time.Second {
		config.PollInterval = pollInterval.String()
		config.pollDuration = *pollInterval
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

	// Log configuration
	logger.Info("Configuration loaded",
		zap.String("baseURL", config.BaseURL),
		zap.String("telco", config.Telco),
		zap.String("entryChannel", config.EntryChannel),
		zap.Strings("entryChannels", config.EntryChannels),
		zap.Strings("productIds", config.ProductIds),
		zap.Strings("msisdns", config.MSISDNS),
		zap.Int("startIndex", config.StartIndex),
		zap.Int("endIndex", config.EndIndex),
		zap.Duration("waitBetweenCalls", config.GetWaitDuration()),
		zap.Duration("pollInterval", config.pollDuration),
		zap.Bool("runOnce", *runOnce),
		zap.Bool("dryRun", *dryRun),
		zap.Bool("metrics", config.EnableMetrics),
		zap.String("metricsAddr", config.MetricsAddr),
		zap.String("timezone", config.Timezone),
		zap.Any("pauseWindows", config.PauseWindows),
		zap.Duration("maxPollingDuration", config.maxPollDur),
	)

	// If dry run mode, just log what would be done and exit
	if *dryRun {
		logger.Info("DRY RUN MODE - Showing what would be executed")
		logger.Info("Would process resubscribe request",
			zap.String("telco", config.Telco),
			zap.Strings("productIds", config.ProductIds),
			zap.String("entryChannel", config.EntryChannel),
			zap.Strings("entryChannels", config.EntryChannels),
			zap.Int("msisdnCount", len(config.MSISDNS)),
			zap.Int("startIndex", config.StartIndex),
			zap.Int("endIndex", config.EndIndex),
		)
		return
	}

	// Create resubscribe processor
	processor := NewResubscribeProcessor(config, logger)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start processing
	processor.Start()

	// Wait for signal
	sig := <-sigChan
	logger.Info("Received signal", zap.String("signal", sig.String()))

	// Stop processor
	processor.Stop()

	logger.Info("Resubscribe processor exited successfully")
}
