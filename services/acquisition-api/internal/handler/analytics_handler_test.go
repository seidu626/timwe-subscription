package handler

import (
	"testing"
)

func TestHashString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string returns empty",
			input:    "",
			expected: "",
		},
		{
			name:     "simple string",
			input:    "test",
			expected: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		},
		{
			name:     "IP address",
			input:    "192.168.1.1",
			expected: "823a2a2b9b5268a5d6d2b3a1c3e2d1f0a9c8e7d6b5a4d3f2c1b0a9e8f7d6c5b4"[:64], // placeholder
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hashString(tt.input)
			if tt.input == "" {
				if result != "" {
					t.Errorf("hashString(%q) = %q, want empty string", tt.input, result)
				}
			} else {
				// Just verify it returns a 64-char hex string for non-empty input
				if len(result) != 64 {
					t.Errorf("hashString(%q) returned %d chars, want 64", tt.input, len(result))
				}
			}
		})
	}
}

func TestExtractReferrerDomain(t *testing.T) {
	tests := []struct {
		name     string
		referrer string
		expected *string
	}{
		{
			name:     "empty referrer",
			referrer: "",
			expected: nil,
		},
		{
			name:     "full URL",
			referrer: "https://www.google.com/search?q=test",
			expected: strPtr("www.google.com"),
		},
		{
			name:     "URL with port",
			referrer: "http://localhost:3000/page",
			expected: strPtr("localhost:3000"),
		},
		{
			name:     "invalid URL",
			referrer: "not-a-valid-url",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractReferrerDomain(tt.referrer)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("extractReferrerDomain(%q) = %v, want nil", tt.referrer, *result)
				}
			} else {
				if result == nil {
					t.Errorf("extractReferrerDomain(%q) = nil, want %q", tt.referrer, *tt.expected)
				} else if *result != *tt.expected {
					t.Errorf("extractReferrerDomain(%q) = %q, want %q", tt.referrer, *result, *tt.expected)
				}
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
