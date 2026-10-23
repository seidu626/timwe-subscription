// Package tenantevents defines the credential shared by the admin API and event sender.
package tenantevents

import (
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"unicode/utf8"
)

// AllowPrivateTargetsEnv lets local development deliver over plain HTTP to loopback or private hosts.
// Production leaves it unset, so a webhook can only reach a public HTTPS host on port 443.
const AllowPrivateTargetsEnv = "TENANT_EVENTS_ALLOW_PRIVATE_URLS"

type Config struct {
	URL             string  `json:"url"`
	SigningSecret   string  `json:"signing_secret"`
	DefaultAmount   float64 `json:"default_amount"`
	DefaultCurrency string  `json:"default_currency"`
}

// AllowPrivateTargets reports whether the local-development escape hatch is on.
func AllowPrivateTargets() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(AllowPrivateTargetsEnv))) {
	case "1", "true", "yes":
		return true
	}
	return false
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
	if err := CheckURL(cfg.URL, AllowPrivateTargets()); err != nil {
		return nil, err
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

// CheckURL accepts only an absolute HTTPS URL on port 443 whose host is a public DNS name or a public IP.
// The dispatcher repeats the address check on every resolved IP when it dials, so DNS cannot point a
// public-looking name at an internal service.
func CheckURL(raw string, allowPrivate bool) error {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return fmt.Errorf("url must be an absolute https URL without credentials or fragment")
	}
	if allowPrivate {
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("url must be an absolute http or https URL")
		}
		return nil
	}
	if u.Scheme != "https" {
		return fmt.Errorf("url must use https")
	}
	if port := u.Port(); port != "" && port != "443" {
		return fmt.Errorf("url must use the default https port")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if addr, err := netip.ParseAddr(host); err == nil {
		if !PublicAddr(addr) {
			return fmt.Errorf("url host must be a public address")
		}
		return nil
	}
	labels := strings.Split(host, ".")
	last := labels[len(labels)-1]
	if len(labels) < 2 || last == "" || strings.Trim(last, "0123456789") == "" {
		return fmt.Errorf("url host must be a fully qualified public domain name")
	}
	for _, suffix := range []string{"localhost", "local", "internal", "lan", "home.arpa", "localdomain"} {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return fmt.Errorf("url host must be a public domain name")
		}
	}
	return nil
}

var nonPublicPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("fec0::/10"),
}

// PublicAddr reports whether addr is a globally routable unicast address. Loopback, private, link-local
// (including cloud metadata), carrier-grade NAT, multicast, documentation, reserved and IPv4-embedding
// IPv6 ranges are refused.
func PublicAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() || addr.Zone() != "" {
		return false
	}
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return false
	}
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}
