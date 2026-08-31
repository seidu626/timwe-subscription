package handler

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestJobStatusNoDataRace guards the concurrency contract of BatchJobManager:
// status writes must be routed through the locked manager helpers and status
// reads serialized via Snapshot, never as raw field reads/writes on the live
// pointer while worker goroutines mutate it. Run with `go test -race` — this
// is a regression test for the worker-vs-poller race on BatchJobStatus fields.
func TestJobStatusNoDataRace(t *testing.T) {
	mgr := NewBatchJobManager()
	jobID := "race-job"
	_, _ = mgr.CreateJob(jobID, 1000)
	mgr.setRunning(jobID)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Worker goroutine: mutate counters through the locked helpers, as
	// runBatchJob / BackfillOptinHandler / ResubscribeHandler now do.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			mgr.SetCounters(jobID, i, int64(i), int64(0))
			mgr.SetTotal(jobID, i)
			if i%3 == 0 {
				mgr.SetErrorDetails(jobID, map[string]interface{}{"error": "x"})
			}
		}
	}()

	// Poller goroutine: read a consistent snapshot and serialize it, exactly as
	// GetBatchProgressHandler / BatchStatusHandler do.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			st, ok := mgr.Snapshot(jobID)
			if !ok {
				continue
			}
			_, err := json.Marshal(st)
			assert.NoError(t, err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()

	// Job should still be tracked and readable after the concurrency churn.
	_, ok := mgr.Snapshot(jobID)
	assert.True(t, ok)
}

// TestPruneCompletedBoundsMap verifies terminal-job eviction never removes
// pending/running jobs.
func TestPruneCompletedBoundsMap(t *testing.T) {
	mgr := NewBatchJobManager()

	mgr.CreateJob("pending-1", 0)
	mgr.CreateJob("running-1", 0)
	mgr.setRunning("running-1")

	// Pending and running jobs must never be pruned, even at the hard cap.
	pruned := mgr.PruneCompleted()
	assert.Zero(t, pruned)
	_, ok := mgr.GetJob("pending-1")
	assert.True(t, ok)
	_, ok = mgr.GetJob("running-1")
	assert.True(t, ok)
}
