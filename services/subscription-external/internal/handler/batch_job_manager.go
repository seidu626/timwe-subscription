package handler

import (
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
)

type BatchJobStatus struct {
	ID           string                 `json:"id"`
	State        BatchJobState          `json:"state"`
	Total        int                    `json:"total"`
	Processed    int64                  `json:"processed"`
	Successful   int64                  `json:"successful"`
	Failed       int64                  `json:"failed"`
	ErrorDetails map[string]interface{} `json:"errorDetails,omitempty"`
	StartedAt    time.Time              `json:"startedAt"`
	CompletedAt  *time.Time             `json:"completedAt,omitempty"`
}

type BatchJobManager struct {
	mu   sync.RWMutex
	jobs map[string]*BatchJobStatus
}

func NewBatchJobManager() *BatchJobManager {
	return &BatchJobManager{jobs: make(map[string]*BatchJobStatus)}
}

func (m *BatchJobManager) CreateJob(id string, total int) *BatchJobStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := &BatchJobStatus{
		ID:        id,
		State:     BatchJobPending,
		Total:     total,
		StartedAt: time.Now(),
	}
	m.jobs[id] = st
	return st
}

func (m *BatchJobManager) GetJob(id string) (*BatchJobStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.jobs[id]
	return st, ok
}

func (m *BatchJobManager) setRunning(id string) {
	m.mu.Lock()
	if st, ok := m.jobs[id]; ok {
		st.State = BatchJobRunning
	}
	m.mu.Unlock()
}

func (m *BatchJobManager) setCompleted(id string, failed bool) {
	m.mu.Lock()
	if st, ok := m.jobs[id]; ok {
		if failed {
			st.State = BatchJobFailed
		} else {
			st.State = BatchJobCompleted
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
