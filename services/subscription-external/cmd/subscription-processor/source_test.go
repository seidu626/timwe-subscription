package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseSource(t *testing.T) {
	for _, tt := range []struct {
		name, format, input string
		want                []string
		duplicates          int
		fail                bool
	}{
		{"text", "text", " +233240000001\r\n\n233240000001\n0240000002", []string{"233240000001", "0240000002"}, 1, false},
		{"csv", "auto", "name,MSISDN\nOne,+233240000001\nTwo,233240000002", []string{"233240000001", "233240000002"}, 0, false},
		{"json array", "auto", `["233240000001", "233240000001"]`, []string{"233240000001"}, 1, false},
		{"json envelope", "json", `{"msisdns":["233240000001"]}`, []string{"233240000001"}, 0, false},
		{"bom", "auto", "\ufeffmsisdn\n233240000001", []string{"233240000001"}, 0, false},
		{"empty", "auto", "  ", nil, 0, true},
		{"header only", "csv", "msisdn", nil, 0, true},
		{"numeric JSON", "json", `[233240000001]`, nil, 0, true},
		{"missing header", "csv", "one,233240000001", nil, 0, true},
		{"duplicate header", "csv", "msisdn,msisdn\n233240000001,233240000002", nil, 0, true},
		{"bad row", "csv", "msisdn,name\n233240000001", nil, 0, true},
		{"scientific notation", "text", "2.3324e11", nil, 0, true},
		{"empty JSON item", "json", `[""]`, nil, 0, true},
		{"too long", "text", "1234567890123456", nil, 0, true},
		{"invalid trailing row", "text", "233240000001\nprivate-value", nil, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, duplicates, err := parseSource([]byte(tt.input), tt.format)
			if (err != nil) != tt.fail {
				t.Fatalf("error=%v want failure=%v", err, tt.fail)
			}
			if err != nil {
				if strings.Contains(err.Error(), "private-value") {
					t.Fatal("error leaked source value")
				}
				return
			}
			if !reflect.DeepEqual(got, tt.want) || duplicates != tt.duplicates {
				t.Fatalf("got %v duplicates=%d", got, duplicates)
			}
		})
	}
}

func TestFileStdinAndHTTPSource(t *testing.T) {
	c := testConfig(t)
	path := filepath.Join(t.TempDir(), "feed.csv")
	if err := os.WriteFile(path, []byte("msisdn\n233240000001\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c.Source = path
	numbers, _, err := readSource(context.Background(), c, http.DefaultClient, nil)
	if err != nil || len(numbers) != 1 {
		t.Fatalf("file: %v %v", numbers, err)
	}
	c.Source = "-"
	if _, _, err := readSource(context.Background(), c, http.DefaultClient, strings.NewReader("233240000001")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_FEED_TOKEN", "feed-test-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer feed-test-token" || r.Header.Get("X-Internal-Signature") != "" {
			t.Error("wrong feed auth")
		}
		io.WriteString(w, `{"msisdns":["233240000001"]}`)
	}))
	defer server.Close()
	c.Source, c.SourceTokenEnv = server.URL, "TEST_FEED_TOKEN"
	if _, _, err := readSource(context.Background(), c, server.Client(), nil); err != nil {
		t.Fatal(err)
	}
	c.MaxSourceBytes = 5
	if _, _, err := readSource(context.Background(), c, server.Client(), nil); err == nil {
		t.Fatal("oversized feed accepted")
	}
}

func TestCLIInvalidInputAndDryRunNeverSubmit(t *testing.T) {
	c := testConfig(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(500) }))
	defer server.Close()
	c.BaseURL, c.Source = server.URL, "-"
	path := writeConfig(t, c)
	t.Setenv("INTERNAL_API_SECRET", "test-secret")
	if err := runCLI(context.Background(), []string{"-config", path}, strings.NewReader("233240000001\ninvalid"), io.Discard); err == nil {
		t.Fatal("invalid source accepted")
	}
	t.Setenv("INTERNAL_API_SECRET", "")
	if err := runCLI(context.Background(), []string{"-config", path, "-dry-run"}, strings.NewReader("233240000001"), io.Discard); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("API called %d times", calls)
	}
	if _, err := os.Stat(c.StateFile); !os.IsNotExist(err) {
		t.Fatal("checkpoint written without subscriptions")
	}
}

func TestHTTPSourceFailureAndRedirectDoNotSubmit(t *testing.T) {
	for _, code := range []int{http.StatusServiceUnavailable, http.StatusFound} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			c := testConfig(t)
			targetCalls := 0
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalls++ }))
			defer target.Close()
			source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", target.URL)
				w.WriteHeader(code)
			}))
			defer source.Close()
			c.Source, c.BaseURL = source.URL, target.URL
			t.Setenv("INTERNAL_API_SECRET", "test-secret")
			if err := runCLI(context.Background(), []string{"-config", writeConfig(t, c)}, nil, io.Discard); err == nil {
				t.Fatal("source failure ignored")
			}
			if targetCalls != 0 {
				t.Fatal("followed redirect or submitted on source error")
			}
		})
	}
}

func TestStdinCancellationReleasesCheckpointLock(t *testing.T) {
	c := testConfig(t)
	path := writeConfig(t, c)
	t.Setenv("INTERNAL_API_SECRET", "test-secret")
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runCLI(ctx, []string{"-config", path}, reader, io.Discard); err == nil {
		t.Fatal("cancelled stdin accepted")
	}
	if _, err := os.Stat(c.StateFile + ".lock"); !os.IsNotExist(err) {
		t.Fatal("checkpoint lock retained after cancellation")
	}
}
