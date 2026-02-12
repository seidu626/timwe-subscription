package monitoring

import (
	"context"
	"database/sql"
	"runtime"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// ComponentHealth represents the health of a specific component
type ComponentHealth struct {
	Name         string                 `json:"name"`
	Status       HealthStatus           `json:"status"`
	LastCheck    time.Time              `json:"last_check"`
	ResponseTime time.Duration          `json:"response_time"`
	Error        string                 `json:"error,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	LastFailure  *time.Time             `json:"last_failure,omitempty"`
	FailureCount int                    `json:"failure_count"`
}

// SystemHealth represents overall system health
type SystemHealth struct {
	OverallStatus HealthStatus                `json:"overall_status"`
	LastCheck     time.Time                   `json:"last_check"`
	Components    map[string]*ComponentHealth `json:"components"`
	Resources     *ResourceUtilization        `json:"resources"`
	Performance   *PerformanceMetrics         `json:"performance"`
	Uptime        time.Duration               `json:"uptime"`
	StartTime     time.Time                   `json:"start_time"`
	mu            sync.RWMutex
}

// ResourceUtilization represents system resource usage
type ResourceUtilization struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	NetworkIO   float64 `json:"network_io"`
	Goroutines  int     `json:"goroutines"`
	HeapAlloc   uint64  `json:"heap_alloc"`
	HeapSys     uint64  `json:"heap_sys"`
	HeapIdle    uint64  `json:"heap_idle"`
	HeapInuse   uint64  `json:"heap_inuse"`
}

// PerformanceMetrics represents system performance indicators
type PerformanceMetrics struct {
	ResponseTime      time.Duration `json:"response_time"`
	Throughput        float64       `json:"throughput"`
	ErrorRate         float64       `json:"error_rate"`
	ActiveConnections int           `json:"active_connections"`
	QueueSize         int           `json:"queue_size"`
	ProcessingRate    float64       `json:"processing_rate"`
}

// HealthChecker interface for components that can report their health
type HealthChecker interface {
	CheckHealth() (*ComponentHealth, error)
	GetName() string
}

// SystemHealthMonitor monitors overall system health
type SystemHealthMonitor struct {
	health    *SystemHealth
	checkers  map[string]HealthChecker
	config    *HealthConfig
	logger    *zap.Logger
	stopChan  chan struct{}
	isRunning bool
	mu        sync.RWMutex
	startTime time.Time
}

// NewSystemHealthMonitor creates a new system health monitor
func NewSystemHealthMonitor(config *HealthConfig, logger *zap.Logger) *SystemHealthMonitor {
	return &SystemHealthMonitor{
		health: &SystemHealth{
			OverallStatus: HealthStatusUnknown,
			Components:    make(map[string]*ComponentHealth),
			Resources:     &ResourceUtilization{},
			Performance:   &PerformanceMetrics{},
			StartTime:     time.Now(),
		},
		checkers:  make(map[string]HealthChecker),
		config:    config,
		logger:    logger,
		stopChan:  make(chan struct{}),
		startTime: time.Now(),
	}
}

// Start begins health monitoring
func (shm *SystemHealthMonitor) Start(ctx context.Context) error {
	if shm.isRunning {
		return nil
	}

	shm.isRunning = true
	shm.logger.Info("Starting system health monitor")

	// Start health checking goroutine
	go shm.healthCheckLoop(ctx)

	// Start resource monitoring goroutine
	go shm.resourceMonitoringLoop(ctx)

	shm.logger.Info("System health monitor started successfully")
	return nil
}

// Stop stops health monitoring
func (shm *SystemHealthMonitor) Stop() {
	if !shm.isRunning {
		return
	}

	shm.logger.Info("Stopping system health monitor")
	close(shm.stopChan)
	shm.isRunning = false
}

// RegisterHealthChecker registers a component for health checking
func (shm *SystemHealthMonitor) RegisterHealthChecker(checker HealthChecker) {
	shm.mu.Lock()
	defer shm.mu.Unlock()

	name := checker.GetName()
	shm.checkers[name] = checker
	shm.logger.Info("Registered health checker", zap.String("component", name))
}

// UnregisterHealthChecker removes a component from health checking
func (shm *SystemHealthMonitor) UnregisterHealthChecker(name string) {
	shm.mu.Lock()
	defer shm.mu.Unlock()

	delete(shm.checkers, name)
	shm.logger.Info("Unregistered health checker", zap.String("component", name))
}

// healthCheckLoop performs periodic health checks
func (shm *SystemHealthMonitor) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(shm.config.CheckInterval)
	defer ticker.Stop()

	// Perform initial health check
	shm.performHealthCheck()

	for {
		select {
		case <-ctx.Done():
			return
		case <-shm.stopChan:
			return
		case <-ticker.C:
			shm.performHealthCheck()
		}
	}
}

// performHealthCheck performs health checks on all registered components
func (shm *SystemHealthMonitor) performHealthCheck() {
	shm.mu.Lock()
	checkers := make(map[string]HealthChecker, len(shm.checkers))
	for k, v := range shm.checkers {
		checkers[k] = v
	}
	shm.mu.Unlock()

	startTime := time.Now()
	overallStatus := HealthStatusHealthy
	componentCount := 0
	healthyCount := 0

	// Check each component
	for name, checker := range checkers {
		componentCount++
		health, err := checker.CheckHealth()
		if err != nil {
			shm.logger.Error("Health check failed",
				zap.String("component", name),
				zap.Error(err))

			// Update component health
			shm.updateComponentHealth(name, &ComponentHealth{
				Name:         name,
				Status:       HealthStatusUnhealthy,
				LastCheck:    time.Now(),
				Error:        err.Error(),
				LastFailure:  &startTime,
				FailureCount: 1,
			})
			overallStatus = HealthStatusUnhealthy
		} else {
			// Update component health
			shm.updateComponentHealth(name, health)
			if health.Status == HealthStatusHealthy {
				healthyCount++
			} else if health.Status == HealthStatusDegraded && overallStatus == HealthStatusHealthy {
				overallStatus = HealthStatusDegraded
			}
		}
	}

	// Update overall health status
	shm.mu.Lock()
	shm.health.OverallStatus = overallStatus
	shm.health.LastCheck = time.Now()
	shm.health.Uptime = time.Since(shm.startTime)
	shm.mu.Unlock()

	// Log health check results
	shm.logger.Info("Health check completed",
		zap.String("overall_status", string(overallStatus)),
		zap.Int("total_components", componentCount),
		zap.Int("healthy_components", healthyCount),
		zap.Duration("check_duration", time.Since(startTime)))
}

// resourceMonitoringLoop monitors system resources
func (shm *SystemHealthMonitor) resourceMonitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second) // Resource monitoring every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-shm.stopChan:
			return
		case <-ticker.C:
			shm.updateResourceUtilization()
		}
	}
}

// updateResourceUtilization updates current resource usage
func (shm *SystemHealthMonitor) updateResourceUtilization() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	shm.mu.Lock()
	shm.health.Resources = &ResourceUtilization{
		Goroutines: runtime.NumGoroutine(),
		HeapAlloc:  m.HeapAlloc,
		HeapSys:    m.HeapSys,
		HeapIdle:   m.HeapIdle,
		HeapInuse:  m.HeapInuse,
		// Note: CPU, Memory, Disk, and Network monitoring would require
		// platform-specific implementations or external tools
	}
	shm.mu.Unlock()
}

// updateComponentHealth updates the health status of a component
func (shm *SystemHealthMonitor) updateComponentHealth(name string, health *ComponentHealth) {
	shm.mu.Lock()
	defer shm.mu.Unlock()

	// Update failure count if this is a failure
	if health.Status == HealthStatusUnhealthy {
		if existing, exists := shm.health.Components[name]; exists {
			health.FailureCount = existing.FailureCount + 1
		}
	}

	shm.health.Components[name] = health
}

// GetHealth returns the current system health status
func (shm *SystemHealthMonitor) GetHealth() *SystemHealth {
	shm.mu.RLock()
	defer shm.mu.RUnlock()

	// Create a copy to avoid external modification
	healthCopy := *shm.health
	healthCopy.Components = make(map[string]*ComponentHealth)

	for k, v := range shm.health.Components {
		componentCopy := *v
		healthCopy.Components[k] = &componentCopy
	}

	return &healthCopy
}

// IsHealthy returns whether the overall system is healthy
func (shm *SystemHealthMonitor) IsHealthy() bool {
	shm.mu.RLock()
	defer shm.mu.RUnlock()
	return shm.health.OverallStatus == HealthStatusHealthy
}

// GetComponentHealth returns the health of a specific component
func (shm *SystemHealthMonitor) GetComponentHealth(name string) *ComponentHealth {
	shm.mu.RLock()
	defer shm.mu.RUnlock()

	if health, exists := shm.health.Components[name]; exists {
		// Return a copy to avoid external modification
		healthCopy := *health
		return &healthCopy
	}
	return nil
}

// ForceHealthCheck triggers an immediate health check
func (shm *SystemHealthMonitor) ForceHealthCheck() {
	shm.logger.Info("Forcing immediate health check")
	shm.performHealthCheck()
}

// DatabaseHealthChecker implements HealthChecker for database connections
type DatabaseHealthChecker struct {
	db     *sql.DB
	name   string
	logger *zap.Logger
}

// NewDatabaseHealthChecker creates a new database health checker
func NewDatabaseHealthChecker(db *sql.DB, name string, logger *zap.Logger) *DatabaseHealthChecker {
	return &DatabaseHealthChecker{
		db:     db,
		name:   name,
		logger: logger,
	}
}

// GetName returns the checker name
func (dhc *DatabaseHealthChecker) GetName() string {
	return dhc.name
}

// CheckHealth performs a database health check
func (dhc *DatabaseHealthChecker) CheckHealth() (*ComponentHealth, error) {
	startTime := time.Now()

	// Simple ping to check database connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := dhc.db.PingContext(ctx)
	responseTime := time.Since(startTime)

	if err != nil {
		return &ComponentHealth{
			Name:         dhc.name,
			Status:       HealthStatusUnhealthy,
			LastCheck:    time.Now(),
			ResponseTime: responseTime,
			Error:        err.Error(),
		}, nil
	}

	return &ComponentHealth{
		Name:         dhc.name,
		Status:       HealthStatusHealthy,
		LastCheck:    time.Now(),
		ResponseTime: responseTime,
		Details: map[string]interface{}{
			"connection_pool_stats": dhc.getConnectionPoolStats(),
		},
	}, nil
}

// getConnectionPoolStats returns database connection pool statistics
func (dhc *DatabaseHealthChecker) getConnectionPoolStats() map[string]interface{} {
	stats := dhc.db.Stats()
	return map[string]interface{}{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration,
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
	}
}

// ServiceHealthChecker implements HealthChecker for general services
type ServiceHealthChecker struct {
	name      string
	checkFunc func() error
	logger    *zap.Logger
}

// NewServiceHealthChecker creates a new service health checker
func NewServiceHealthChecker(name string, checkFunc func() error, logger *zap.Logger) *ServiceHealthChecker {
	return &ServiceHealthChecker{
		name:      name,
		checkFunc: checkFunc,
		logger:    logger,
	}
}

// GetName returns the checker name
func (shc *ServiceHealthChecker) GetName() string {
	return shc.name
}

// CheckHealth performs a service health check
func (shc *ServiceHealthChecker) CheckHealth() (*ComponentHealth, error) {
	startTime := time.Now()

	err := shc.checkFunc()
	responseTime := time.Since(startTime)

	if err != nil {
		return &ComponentHealth{
			Name:         shc.name,
			Status:       HealthStatusUnhealthy,
			LastCheck:    time.Now(),
			ResponseTime: responseTime,
			Error:        err.Error(),
		}, nil
	}

	return &ComponentHealth{
		Name:         shc.name,
		Status:       HealthStatusHealthy,
		LastCheck:    time.Now(),
		ResponseTime: responseTime,
	}, nil
}
