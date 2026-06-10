package pii

import (
	"strings"
	"testing"
)

func TestMaskMSISDN(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "typical 12-digit MSISDN",
			input: "233241234567",
			want:  "23324*****67",
		},
		{
			name:  "11-digit MSISDN",
			input: "23324123456",
			want:  "23324****56",
		},
		{
			name:  "exactly keepPrefix+keepSuffix (7 chars) — too short",
			input: "1234567",
			want:  "***",
		},
		{
			name:  "short number (3 chars)",
			input: "123",
			want:  "***",
		},
		{
			name:  "empty string",
			input: "",
			want:  "***",
		},
		{
			// A value already masked still gets re-masked (stars are opaque chars).
			// Input "23324****67" is 11 chars → keepPrefix=5, keepSuffix=2, middle=4 → "23324****67".
			name:  "already-masked value passes through masked form",
			input: "23324****67",
			want:  "23324****67",
		},
		{
			name:  "minimum maskable length (8 chars)",
			input: "12345678",
			want:  "12345*78",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MaskMSISDN(tc.input)
			if got != tc.want {
				t.Errorf("MaskMSISDN(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestMaskMSISDN_PreservesCountryPrefix(t *testing.T) {
	msisdn := "233241234567"
	masked := MaskMSISDN(msisdn)
	if !strings.HasPrefix(masked, "23324") {
		t.Errorf("expected masked MSISDN to start with country prefix, got %q", masked)
	}
}

func TestMaskMSISDN_HasStars(t *testing.T) {
	masked := MaskMSISDN("233241234567")
	if !strings.Contains(masked, "*") {
		t.Errorf("expected masked MSISDN to contain '*', got %q", masked)
	}
}

func TestMaskMSISDN_DoesNotLeakMiddle(t *testing.T) {
	input := "233241234567"
	masked := MaskMSISDN(input)
	// Middle portion "1234" must not appear verbatim.
	if strings.Contains(masked, "1234") {
		t.Errorf("masked MSISDN leaks middle digits: %q", masked)
	}
}
