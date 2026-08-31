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

// batchJobRetention and batchJobHardCap govern eviction of terminal jobs so the
// jobs map cannot grow without bound in a long-lived server.
const (
	batchJobRetention = 24 * time.Hour
	batchJobHardCap   = 10000
)

// PruneCompleted removes terminal (completed/failed/cancelled) jobs. Jobs are
// kept for batchJobRetention to give pollers time to read their results; if the
// map still exceeds batchJobHardCap, the oldest terminal jobs are evicted
// regardless of age. Pending and running jobs are never evicted.
func (m *BatchJobManager) PruneCompleted() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.jobs) <= batchJobHardCap {
		return 0
	}
	cutoff := time.Now().Add(-batchJobRetention)
	evicted := 0
	for id, entry := range m.jobs {
		switch entry.status.State {
		case BatchJobCompleted, BatchJobFailed, BatchJobCancelled:
			if entry.status.CompletedAt != nil && entry.status.CompletedAt.Before(cutoff) {
				delete(m.jobs, id)
				evicted++
			}
		}
	}
	// Hard cap: if still over the limit, evict the oldest terminal jobs.
	for len(m.jobs) > batchJobHardCap {
		var oldestID string
		var oldest time.Time
		found := false
		for id, entry := range m.jobs {
			switch entry.status.State {
			case BatchJobCompleted, BatchJobFailed, BatchJobCancelled:
				at := time.Time{}
				if entry.status.CompletedAt != nil {
					at = *entry.status.CompletedAt
				}
				if !found || at.Before(oldest) {
					oldestID, oldest, found = id, at, true
				}
			}
		}
		if !found {
			break // only live jobs remain; do not evict them
		}
		delete(m.jobs, oldestID)
		evicted++
	}
	return evicted
}

// CreateJob creates a new job entry. Returns the status and a context that is
// cancelled when CancelJob is called for this id.
func (m *BatchJobManager) CreateJob(id string, total int) (*BatchJobStatus, context.Context) {
	m.PruneCompleted()
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
	m.PruneCompleted()
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

// Snapshot returns a deep copy of the job status captured while holding the
// manager lock. Consumers (e.g. HTTP status handlers that serialize to JSON)
// must use this instead of GetJob + direct field reads, because GetJob returns
// the live pointer and worker goroutines mutate its fields concurrently.
func (m *BatchJobManager) Snapshot(id string) (*BatchJobStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.jobs[id]
	if !ok {
		return nil, false
	}
	src := entry.status
	if src == nil {
		return nil, false
	}
	cp := *src
	if src.ErrorDetails != nil {
		cp.ErrorDetails = make(map[string]interface{}, len(src.ErrorDetails))
		for k, v := range src.ErrorDetails {
			cp.ErrorDetails[k] = v
		}
	}
	if src.CompletedAt != nil {
		t := *src.CompletedAt
		cp.CompletedAt = &t
	}
	return &cp, true
}

// SetCounters updates a job's terminal counters (Success/Failed) and Total
// under the manager lock. Callers that mutate status fields concurrently with
// status polling must use these helpers so reads under the lock stay race-free.
func (m *BatchJobManager) SetCounters(id string, total int, success, failed int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.jobs[id]
	if !ok {
		return false
	}
	entry.status.Total = total
	entry.status.Successful = success
	entry.status.Failed = failed
	return true
}

// SetTotal updates a job's Total under the manager lock.
func (m *BatchJobManager) SetTotal(id string, total int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.jobs[id]
	if !ok {
		return false
	}
	entry.status.Total = total
	return true
}

// Owner returns the owning tenant key for a job, read under the manager lock.
func (m *BatchJobManager) Owner(id string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.jobs[id]
	if !ok {
		return "", false
	}
	return entry.status.TenantKey, true
}

// Count returns the number of tracked jobs (for health/lifecycle reporting).
func (m *BatchJobManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.jobs)
}

// SetErrorDetails records terminal error details under the manager lock.
func (m *BatchJobManager) SetErrorDetails(id string, details map[string]interface{}) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.jobs[id]
	if !ok {
		return false
	}
	entry.status.ErrorDetails = details
	return true
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
