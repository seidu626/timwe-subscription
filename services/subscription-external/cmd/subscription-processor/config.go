package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config describes a finite input snapshot and its subscription route.
type Config struct {
	BaseURL                      string   `json:"base_url"`
	Source                       string   `json:"source"`
	Format                       string   `json:"format"`
	SourceTokenEnv               string   `json:"source_token_env,omitempty"`
	DatabaseURLEnv               string   `json:"database_url_env,omitempty"`
	DatabaseQuery                string   `json:"database_query,omitempty"`
	MaxSourceBytes               int64    `json:"max_source_bytes"`
	BatchSize                    int      `json:"batch_size"`
	Telco                        string   `json:"telco"`
	EntryChannel                 string   `json:"entry_channel"`
	ProductIDs                   []string `json:"product_ids"`
	TenantKey                    string   `json:"tenant_key"`
	ChannelKey                   string   `json:"channel_key"`
	WaitBetweenCalls             string   `json:"wait_between_calls"`
	PollInterval                 string   `json:"poll_interval"`
	MaxPollingDuration           string   `json:"max_polling_duration"`
	RequestTimeout               string   `json:"request_timeout"`
	StateFile                    string   `json:"state_file"`
	wait, poll, maxPoll, timeout time.Duration
}

func loadConfig(path string) (Config, error) {
	c := Config{Format: "auto", MaxSourceBytes: 64 << 20, BatchSize: 100,
		WaitBetweenCalls: "1s", PollInterval: "2s", MaxPollingDuration: "30m",
		RequestTimeout: "30s", StateFile: "subscription-progress.json"}
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(data, &c)
	return c, err
}

func (c *Config) validate() error {
	for name, value := range map[string]string{"base_url": c.BaseURL, "source": c.Source,
		"tenant_key": c.TenantKey, "channel_key": c.ChannelKey, "telco": c.Telco,
		"entry_channel": c.EntryChannel, "state_file": c.StateFile} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("base_url must be an HTTP(S) URL without credentials, query or fragment")
	}
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	if c.Source == "database" && (strings.TrimSpace(c.DatabaseURLEnv) == "" || strings.TrimSpace(c.DatabaseQuery) == "") {
		return fmt.Errorf("database source requires database_url_env and database_query")
	}
	if c.BatchSize <= 0 || c.MaxSourceBytes <= 0 {
		return fmt.Errorf("batch_size and max_source_bytes must be positive")
	}
	if len(c.ProductIDs) == 0 {
		return fmt.Errorf("product_ids is required")
	}
	for _, id := range c.ProductIDs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("product_ids cannot contain empty values")
		}
	}
	switch c.Format {
	case "auto", "text", "csv", "json":
	default:
		return fmt.Errorf("format must be auto, text, csv or json")
	}
	for _, d := range []struct {
		name, value string
		target      *time.Duration
		zero        bool
	}{
		{"wait_between_calls", c.WaitBetweenCalls, &c.wait, true},
		{"poll_interval", c.PollInterval, &c.poll, false},
		{"max_polling_duration", c.MaxPollingDuration, &c.maxPoll, false},
		{"request_timeout", c.RequestTimeout, &c.timeout, false},
	} {
		duration, err := time.ParseDuration(d.value)
		if err != nil || duration < 0 || (!d.zero && duration == 0) {
			return fmt.Errorf("invalid %s duration", d.name)
		}
		*d.target = duration
	}
	return nil
}
