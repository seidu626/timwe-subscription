package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
)

func failureFor(job string, index int, number string) domain.BatchItemFailure {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", job, index, number)))
	return domain.BatchItemFailure{ItemIndex: index, IdentityHash: hex.EncodeToString(sum[:]), Code: "SUBSCRIPTION_PERSISTENCE_FAILED", Message: "Provider accepted the request; local persistence failed", AcceptedByProvider: true, ProviderTransactionID: "provider-transaction", ExternalTransactionID: "provider-attempt", TrackingID: "local-tracking", ProviderAcceptedAt: "2026-09-19T07:00:00Z", PersistenceAttempts: 3, PostgresCode: "53300"}
}

func TestLargeFailureReceiptPreservesEveryItem(t *testing.T) {
	c := testConfig(t)
	c.BatchSize = 5000
	c.maxPoll = 5 * time.Second
	numbers := make([]string, 5000)
	status := jobStatus{ID: "large-job", State: "failed", Total: 5000, Processed: 5000, Failed: 5000}
	for i := range numbers {
		numbers[i] = fmt.Sprintf("23324%07d", i)
		failure := failureFor(status.ID, i, numbers[i])
		failure.Message = strings.Repeat("local persistence unavailable; ", 12)
		status.Failures = append(status.Failures, failure)
	}
	body, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) <= 1<<20 {
		t.Fatal("fixture must exercise more than the old 1 MiB response limit")
	}
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
			t.Error("resumed job must not enqueue")
		}
		w.Write(body)
	}))
	defer server.Close()
	c.BaseURL = server.URL
	cp, err := loadCheckpoint(c, numbers)
	if err != nil {
		t.Fatal(err)
	}
	cp.JobID = status.ID
	if err := saveCheckpoint(c.StateFile, cp); err != nil {
		t.Fatal(err)
	}
	p := processor{config: c, client: server.Client(), secret: "test", output: io.Discard}
	err = p.run(context.Background(), numbers, cp)
	if err == nil || !strings.Contains(err.Error(), "5000 subscription attempts failed") {
		t.Fatalf("unexpected result: %v", err)
	}
	if cp.Next != 5000 || cp.JobID != "" || posts != 0 {
		t.Fatalf("bad completion: %+v posts=%d", cp, posts)
	}
	path := c.StateFile + ".batch-1-5000.json"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var receipt batchReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	if len(receipt.Job.Failures) != 5000 || receipt.Job.Failures[4999].IdentityHash != status.Failures[4999].IdentityHash || receipt.Start != 0 || receipt.End != 5000 || receipt.Fingerprint != cp.Fingerprint {
		t.Fatal("incomplete receipt")
	}
	if !receipt.Job.Failures[0].AcceptedByProvider || receipt.Job.Failures[0].ProviderTransactionID != "provider-transaction" {
		t.Fatal("provider reconciliation metadata lost")
	}
	if receipt.Job.Failures[0].ExternalTransactionID != "provider-attempt" || receipt.Job.Failures[0].TrackingID != "local-tracking" || receipt.Job.Failures[0].ProviderAcceptedAt != "2026-09-19T07:00:00Z" {
		t.Fatal("provider attempt, local tracking or acceptance time lost")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("receipt permissions expose private data: %v", info.Mode())
	}
	if strings.Contains(string(data), numbers[0]) {
		t.Fatal("receipt includes raw source identifier")
	}
}

func TestReceiptWriteFailureRetainsJobAndResumesWithoutPOST(t *testing.T) {
	c := testConfig(t)
	numbers := []string{"233240000001"}
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
			t.Error("must not enqueue an accepted job")
		}
		json.NewEncoder(w).Encode(jobStatus{ID: "accepted-job", State: "completed", Total: 1, Processed: 1, Successful: 1})
	}))
	defer server.Close()
	c.BaseURL = server.URL
	cp, err := loadCheckpoint(c, numbers)
	if err != nil {
		t.Fatal(err)
	}
	cp.JobID = "accepted-job"
	if err := saveCheckpoint(c.StateFile, cp); err != nil {
		t.Fatal(err)
	}
	path := c.StateFile + ".batch-1-1.json"
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	p := processor{config: c, client: server.Client(), secret: "test", output: io.Discard}
	if err := p.run(context.Background(), numbers, cp); err == nil || !strings.Contains(err.Error(), "receipt could not be saved") {
		t.Fatalf("expected receipt failure, got %v", err)
	}
	saved, err := loadCheckpoint(c, numbers)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Next != 0 || saved.JobID != "accepted-job" || posts != 0 {
		t.Fatal("checkpoint advanced or job replayed on receipt failure")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := p.run(context.Background(), numbers, saved); err != nil {
		t.Fatal(err)
	}
	if posts != 0 || saved.Next != 1 {
		t.Fatal("resume did not exclusively poll/finish")
	}
}

func TestFailureReceiptValidation(t *testing.T) {
	numbers := []string{"233240000001", "233240000002"}
	valid := failureFor("job", 0, numbers[0])
	for _, kind := range []string{"missing", "wrong-hash", "out-of-range", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			status := jobStatus{ID: "job", Failed: 1, Failures: []domain.BatchItemFailure{valid}}
			switch kind {
			case "missing":
				status.Failures = nil
			case "wrong-hash":
				status.Failures[0].IdentityHash = "wrong"
			case "out-of-range":
				status.Failures[0].ItemIndex = 2
			case "duplicate":
				status.Failed = 2
				status.Failures = append(status.Failures, valid)
			}
			if err := validateFailures(status, numbers); err == nil {
				t.Fatal("invalid recovery evidence accepted")
			}
		})
	}
}

func TestInconsistentTerminalStatusRetainsCheckpointWithoutReceipt(t *testing.T) {
	for _, kind := range []string{"underprocessed", "overprocessed", "completed-with-failure", "failed-without-failure"} {
		t.Run(kind, func(t *testing.T) {
			c := testConfig(t)
			numbers := []string{"233240000001"}
			status := jobStatus{ID: "job", State: "completed", Total: 1, Processed: 1, Successful: 1}
			switch kind {
			case "underprocessed":
				status.Processed = 0
			case "overprocessed":
				status.Processed = 2
			case "completed-with-failure":
				status.Successful, status.Failed = 0, 1
				status.Failures = []domain.BatchItemFailure{failureFor(status.ID, 0, numbers[0])}
			case "failed-without-failure":
				status.State = "failed"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Error("accepted job must not be replayed")
				}
				json.NewEncoder(w).Encode(status)
			}))
			defer server.Close()
			c.BaseURL = server.URL
			cp, err := loadCheckpoint(c, numbers)
			if err != nil {
				t.Fatal(err)
			}
			cp.JobID = status.ID
			if err := saveCheckpoint(c.StateFile, cp); err != nil {
				t.Fatal(err)
			}
			p := processor{config: c, client: server.Client(), secret: "test", output: io.Discard}
			if err := p.run(context.Background(), numbers, cp); err == nil || !strings.Contains(err.Error(), "inconsistent totals") {
				t.Fatalf("expected inconsistent status rejection: %v", err)
			}
			saved, err := loadCheckpoint(c, numbers)
			if err != nil || saved.Next != 0 || saved.JobID != "job" {
				t.Fatalf("checkpoint changed: %+v %v", saved, err)
			}
			if _, err := os.Stat(c.StateFile + ".batch-1-1.json"); !os.IsNotExist(err) {
				t.Fatalf("invalid terminal receipt saved: %v", err)
			}
		})
	}
}

func TestCancelledBatchSavesPartialReceiptWithoutAdvancing(t *testing.T) {
	c := testConfig(t)
	c.BatchSize = 3
	numbers := []string{"233240000001", "233240000002", "233240000003"}
	status := jobStatus{ID: "cancelled-job", State: "cancelled", Total: 3, Processed: 2, Successful: 1, Failed: 1,
		Failures: []domain.BatchItemFailure{failureFor("cancelled-job", 1, numbers[1])}}
	gets := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Error("cancelled batch must not be replayed")
		}
		gets++
		response := status
		if gets == 1 {
			response.State = "cancelling"
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	c.BaseURL = server.URL
	cp, err := loadCheckpoint(c, numbers)
	if err != nil {
		t.Fatal(err)
	}
	cp.JobID = status.ID
	if err := saveCheckpoint(c.StateFile, cp); err != nil {
		t.Fatal(err)
	}
	p := processor{config: c, client: server.Client(), secret: "test", output: io.Discard}
	if err := p.run(context.Background(), numbers, cp); err == nil || !strings.Contains(err.Error(), "receipt saved and checkpoint retained") {
		t.Fatalf("unexpected cancellation: %v", err)
	}
	saved, err := loadCheckpoint(c, numbers)
	if err != nil || saved.Next != 0 || saved.JobID != status.ID {
		t.Fatalf("cancelled checkpoint changed: %+v %v", saved, err)
	}
	data, err := os.ReadFile(c.StateFile + ".batch-1-3.json")
	if err != nil {
		t.Fatal(err)
	}
	var receipt batchReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Job.State != "cancelled" || receipt.Job.Processed != 2 || len(receipt.Job.Failures) != 1 || receipt.Job.Failures[0].ItemIndex != 1 {
		t.Fatal("partial cancellation evidence lost")
	}
}
