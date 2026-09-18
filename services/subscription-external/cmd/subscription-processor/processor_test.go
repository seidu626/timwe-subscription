package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
)

func testConfig(t *testing.T) Config {
	t.Helper()
	c := Config{BaseURL: "http://localhost:8080", Source: "-", Format: "auto", MaxSourceBytes: 4096,
		BatchSize: 2, Telco: "MTN", EntryChannel: "USSD", ProductIDs: []string{"test-product"},
		TenantKey: "test-tenant", ChannelKey: "test-channel", WaitBetweenCalls: "0s",
		PollInterval: "1ms", MaxPollingDuration: "1s", RequestTimeout: "1s",
		StateFile: filepath.Join(t.TempDir(), "progress.json")}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	return c
}

func writeConfig(t *testing.T, c Config) string {
	t.Helper()
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCLIChunksExplicitNumbersAndResumesCompletedFeed(t *testing.T) {
	c := testConfig(t)
	var requests []domain.BatchOptinRequest
	polls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mac := hmac.New(sha256.New, []byte("test-secret"))
		mac.Write(append([]byte(r.Header.Get("X-Internal-Timestamp")), body...))
		if r.Header.Get("X-Internal-Signature") != hex.EncodeToString(mac.Sum(nil)) {
			t.Error("missing or invalid HMAC")
		}
		if _, err := time.Parse(time.RFC3339, r.Header.Get("X-Internal-Timestamp")); err != nil {
			t.Error("invalid timestamp")
		}
		if r.URL.Path != "/api/v1/subscription-external/batch" {
			t.Errorf("wrong endpoint %s", r.URL.Path)
		}
		if r.Method == http.MethodPost {
			var req domain.BatchOptinRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Error(err)
			}
			if len(req.MSISDNS) == 0 || req.Count != len(req.MSISDNS) || req.TenantKey != c.TenantKey || req.ChannelKey != c.ChannelKey || req.EntryChannel != c.EntryChannel || req.Telco != c.Telco || !reflect.DeepEqual(req.ProductIds, c.ProductIDs) {
				t.Errorf("wrong request: %+v", req)
			}
			requests = append(requests, req)
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprintf(w, `{"jobId":"job-%d"}`, len(requests))
			return
		}
		polls++
		id := r.URL.Query().Get("jobId")
		count := len(requests[len(requests)-1].MSISDNS)
		json.NewEncoder(w).Encode(jobStatus{ID: id, State: "completed", Total: count, Successful: count})
	}))
	defer server.Close()
	c.BaseURL = server.URL
	path := writeConfig(t, c)
	t.Setenv("INTERNAL_API_SECRET", "test-secret")
	feed := "233240000001\n+233240000001\n233240000002\n233240000003"
	for i := 0; i < 2; i++ {
		if err := runCLI(context.Background(), []string{"-config", path}, strings.NewReader(feed), io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	if len(requests) != 2 || polls != 2 {
		t.Fatalf("posts=%d polls=%d", len(requests), polls)
	}
	got := append(requests[0].MSISDNS, requests[1].MSISDNS...)
	if !reflect.DeepEqual(got, []string{"233240000001", "233240000002", "233240000003"}) {
		t.Fatalf("wrong feed %v", got)
	}
}

func TestPollingFailureResumesWithoutReenqueue(t *testing.T) {
	c := testConfig(t)
	posts, polls := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			posts++
			w.WriteHeader(202)
			io.WriteString(w, `{"jobId":"accepted"}`)
			return
		}
		polls++
		if polls == 1 {
			w.WriteHeader(503)
			return
		}
		io.WriteString(w, `{"id":"accepted","state":"completed","total":1,"successful":1,"failed":0}`)
	}))
	defer server.Close()
	c.BaseURL = server.URL
	path := writeConfig(t, c)
	t.Setenv("INTERNAL_API_SECRET", "test-secret")
	if err := runCLI(context.Background(), []string{"-config", path}, strings.NewReader("233240000001"), io.Discard); err == nil {
		t.Fatal("expected polling failure")
	}
	if err := runCLI(context.Background(), []string{"-config", path}, strings.NewReader("233240000001"), io.Discard); err != nil {
		t.Fatal(err)
	}
	if posts != 1 || polls != 2 {
		t.Fatalf("posts=%d polls=%d", posts, polls)
	}
}

func TestUnknownEnqueueOutcomeNeverReplays(t *testing.T) {
	c := testConfig(t)
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { posts++; w.WriteHeader(202); io.WriteString(w, `{}`) }))
	defer server.Close()
	c.BaseURL = server.URL
	path := writeConfig(t, c)
	t.Setenv("INTERNAL_API_SECRET", "test-secret")
	for i := 0; i < 2; i++ {
		if err := runCLI(context.Background(), []string{"-config", path}, strings.NewReader("233240000001"), io.Discard); err == nil {
			t.Fatal("unknown enqueue accepted")
		}
	}
	if posts != 1 {
		t.Fatalf("replayed POST: %d", posts)
	}
}

func TestCheckpointRejectsChangedSourceRouteAndCorruption(t *testing.T) {
	c := testConfig(t)
	numbers := []string{"233240000001"}
	state, err := loadCheckpoint(c, numbers)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveCheckpoint(c.StateFile, state); err != nil {
		t.Fatal(err)
	}
	changed := c
	changed.TenantKey = "different"
	if _, err := loadCheckpoint(changed, numbers); err == nil {
		t.Fatal("changed tenant accepted")
	}
	if _, err := loadCheckpoint(c, []string{"233240000002"}); err == nil {
		t.Fatal("changed feed accepted")
	}
	if err := os.WriteFile(c.StateFile, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCheckpoint(c, numbers); err == nil {
		t.Fatal("empty checkpoint accepted")
	}
}

func TestCheckpointExcludesConcurrentRuns(t *testing.T) {
	c := testConfig(t)
	unlock, err := lockCheckpoint(c.StateFile)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err := lockCheckpoint(c.StateFile); err == nil {
		t.Fatal("concurrent run accepted")
	}
}

func TestTerminalStatesAndCancellation(t *testing.T) {
	for _, state := range []string{"failed", "cancelled", "unexpected", "running", "inconsistent"} {
		t.Run(state, func(t *testing.T) {
			c := testConfig(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				status := jobStatus{ID: "job", State: state, Total: 1, Failed: 1}
				if state == "inconsistent" {
					status.State = "completed"
					status.Total = 0
				}
				json.NewEncoder(w).Encode(status)
			}))
			defer server.Close()
			c.BaseURL = server.URL
			numbers := []string{"233240000001"}
			cp, err := loadCheckpoint(c, numbers)
			if err != nil {
				t.Fatal(err)
			}
			cp.JobID = "job"
			p := processor{config: c, client: server.Client(), secret: "test", output: io.Discard}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			if err := p.run(ctx, numbers, cp); err == nil {
				t.Fatal("failure was reported as success")
			}
			if state == "failed" && cp.Next != 1 {
				t.Fatal("fully accounted failed batch not recorded")
			}
			if state != "failed" && cp.Next != 0 {
				t.Fatal("unfinished batch advanced")
			}
		})
	}
}
