package handler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type BatchJobState string

const (
	BatchJobPending   BatchJobState = "pending"
	BatchJobRunning   BatchJobState = "running"
	BatchJobCompleted BatchJobState = "completed"
	BatchJobFailed    BatchJobState = "failed"
	BatchJobCancelled BatchJobState = "cancelled"
)

type BatchJobStatus struct {
	ID           string                 `json:"id"`
	State        BatchJobState          `json:"state"`
	Total        int                    `json:"total"`
	Processed    int64                  `json:"processed"`
	Successful   int64                  `json:"successful"`
	Failed       int64                  `json:"failed"`
	TenantKey    string                 `json:"tenantKey,omitempty"`
	ChannelKey   string                 `json:"channelKey,omitempty"`
	ErrorDetails map[string]interface{} `json:"errorDetails,omitempty"`
	StartedAt    time.Time              `json:"startedAt"`
	CompletedAt  *time.Time             `json:"completedAt,omitempty"`
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
	return entry.status, true
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
	entry.cancel()
	entry.status.State = BatchJobCancelled
	now := time.Now()
	entry.status.CompletedAt = &now
	return true
}

func (m *BatchJobManager) setRunning(id string) {
	m.mu.Lock()
	if entry, ok := m.jobs[id]; ok {
		entry.status.State = BatchJobRunning
	}
	m.mu.Unlock()
}

func (m *BatchJobManager) setCompleted(id string, failed bool) {
	m.mu.Lock()
	if entry, ok := m.jobs[id]; ok {
		st := entry.status
		// Don't overwrite a cancelled state.
		if st.State != BatchJobCancelled {
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
