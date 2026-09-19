package handler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
)

type BatchJobState string

const (
	BatchJobPending    BatchJobState = "pending"
	BatchJobRunning    BatchJobState = "running"
	BatchJobCancelling BatchJobState = "cancelling"
	BatchJobCompleted  BatchJobState = "completed"
	BatchJobFailed     BatchJobState = "failed"
	BatchJobCancelled  BatchJobState = "cancelled"
)

type BatchJobStatus struct {
	ID           string                    `json:"id"`
	State        BatchJobState             `json:"state"`
	Total        int                       `json:"total"`
	Processed    int64                     `json:"processed"`
	Successful   int64                     `json:"successful"`
	Failed       int64                     `json:"failed"`
	TenantKey    string                    `json:"tenantKey,omitempty"`
	ChannelKey   string                    `json:"channelKey,omitempty"`
	ErrorDetails map[string]interface{}    `json:"errorDetails,omitempty"`
	Failures     []domain.BatchItemFailure `json:"failures,omitempty"`
	StartedAt    time.Time                 `json:"startedAt"`
	CompletedAt  *time.Time                `json:"completedAt,omitempty"`
}

type batchJobEntry struct {
	status *BatchJobStatus
	cancel context.CancelFunc
}

type BatchJobManager struct {
	mu   sync.RWMutex
	jobs map[string]*batchJobEntry
}

func NewBatchJobManager() *BatchJobManager {
	return &BatchJobManager{jobs: make(map[string]*batchJobEntry)}
}

// CreateJob creates a new job entry. Returns the status and a context that is
// cancelled when CancelJob is called for this id.
func (m *BatchJobManager) CreateJob(id string, total int) (*BatchJobStatus, context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	st := &BatchJobStatus{
		ID:        id,
		State:     BatchJobPending,
		Total:     total,
		StartedAt: time.Now(),
	}
	m.jobs[id] = &batchJobEntry{status: st, cancel: cancel}
	return st, ctx
}

// CreateJobWithTenant registers a new job and stamps tenant ownership atomically
// inside the manager lock (FIX 2), returning both the status record and a
// cancellable context (FIX 1).  The context is cancelled by CancelJob /
// StopBatchHandler so that BackfillOptinHandler / ResubscribeHandler goroutines
// exit on their next select iteration.
func (m *BatchJobManager) CreateJobWithTenant(id string, total int, tenantKey, channelKey string) (*BatchJobStatus, context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	st := &BatchJobStatus{
		ID:         id,
		State:      BatchJobPending,
		Total:      total,
		StartedAt:  time.Now(),
		TenantKey:  tenantKey,
		ChannelKey: channelKey,
	}
	m.jobs[id] = &batchJobEntry{status: st, cancel: cancel}
	return st, ctx
}

func (m *BatchJobManager) GetJob(id string) (*BatchJobStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.jobs[id]
	if !ok {
		return nil, false
	}
	st := entry.status
	return &BatchJobStatus{
		ID: st.ID, State: st.State, Total: st.Total,
		Processed: atomic.LoadInt64(&st.Processed), Successful: atomic.LoadInt64(&st.Successful), Failed: atomic.LoadInt64(&st.Failed),
		TenantKey: st.TenantKey, ChannelKey: st.ChannelKey, ErrorDetails: st.ErrorDetails,
		Failures: append([]domain.BatchItemFailure(nil), st.Failures...), StartedAt: st.StartedAt, CompletedAt: st.CompletedAt,
	}, true
}

// CancelJob cancels a running or pending job by triggering context cancellation.
// Returns true if the job existed and was in a cancellable state.
func (m *BatchJobManager) CancelJob(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.jobs[id]
	if !ok {
		return false
	}
	if entry.status.State != BatchJobRunning && entry.status.State != BatchJobPending {
		return false
	}
	entry.status.State = BatchJobCancelling
	entry.status.CompletedAt = nil
	entry.cancel()
	return true
}

func (m *BatchJobManager) setRunning(id string) {
	m.mu.Lock()
	if entry, ok := m.jobs[id]; ok && entry.status.State == BatchJobPending {
		entry.status.State = BatchJobRunning
	}
	m.mu.Unlock()
}

func (m *BatchJobManager) setCompleted(id string, failed bool) {
	m.mu.Lock()
	if entry, ok := m.jobs[id]; ok {
		st := entry.status
		if st.State == BatchJobCancelling || st.State == BatchJobCancelled {
			st.State = BatchJobCancelled
		} else {
			if failed {
				st.State = BatchJobFailed
			} else {
				st.State = BatchJobCompleted
			}
		}
		now := time.Now()
		st.CompletedAt = &now
	}
	m.mu.Unlock()
}

// Helpers for atomic counters
func (st *BatchJobStatus) incProcessed() { atomic.AddInt64(&st.Processed, 1) }
func (st *BatchJobStatus) incSuccess()   { atomic.AddInt64(&st.Successful, 1) }
func (st *BatchJobStatus) incFailed()    { atomic.AddInt64(&st.Failed, 1) }

func (m *BatchJobManager) setTotal(id string, total int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.jobs[id]; ok {
		entry.status.Total = total
	}
}

func (m *BatchJobManager) setErrorDetails(id string, details map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.jobs[id]; ok {
		entry.status.ErrorDetails = details
	}
}

func (m *BatchJobManager) addFailure(id string, failure domain.BatchItemFailure) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.jobs[id]; ok {
		entry.status.Failures = append(entry.status.Failures, failure)
	}
}
