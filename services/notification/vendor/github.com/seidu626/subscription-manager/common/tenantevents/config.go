// Package tenantevents defines the credential shared by the admin API and event sender.
package tenantevents

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"unicode/utf8"
)

type Config struct {
	URL             string  `json:"url"`
	SigningSecret   string  `json:"signing_secret"`
	DefaultAmount   float64 `json:"default_amount"`
	DefaultCurrency string  `json:"default_currency"`
}

func Parse(blob []byte) (*Config, error) {
	var cfg Config
	decoder := json.NewDecoder(strings.NewReader(string(blob)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("invalid tenant event webhook config")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("tenant event webhook config must contain one JSON object")
	}
	u, err := url.Parse(cfg.URL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
		return nil, fmt.Errorf("url must be an absolute http or https URL without credentials or fragment")
	}
	if utf8.RuneCountInString(cfg.SigningSecret) < 16 {
		return nil, fmt.Errorf("signing_secret must be at least 16 characters")
	}
	if cfg.DefaultAmount <= 0 {
		return nil, fmt.Errorf("default_amount must be greater than 0")
	}
	if len(cfg.DefaultCurrency) != 3 || strings.IndexFunc(cfg.DefaultCurrency, func(r rune) bool { return r < 'A' || r > 'Z' }) >= 0 {
		return nil, fmt.Errorf("default_currency must be three uppercase letters")
	}
	return &cfg, nil
}
