package repository

import (
	"testing"
	"time"
)

func TestNextChargeHint(t *testing.T) {
	// Fixed clock: hints must be deterministic, so now is always explicit.
	now := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		lastCharge   time.Time
		billingCycle string
		want         string
	}{
		{
			name:         "daily charge earlier today renders tomorrow",
			lastCharge:   time.Date(2026, time.August, 24, 6, 0, 0, 0, time.UTC),
			billingCycle: "DAILY",
			want:         "Renews 25 Aug",
		},
		{
			name:         "weekly advances past now in whole cycles",
			lastCharge:   time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC),
			billingCycle: "weekly",
			want:         "Renews 31 Aug",
		},
		{
			name:         "biweekly from a stale charge lands on the next future boundary",
			lastCharge:   time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
			billingCycle: "BIWEEKLY",
			want:         "Renews 7 Sep",
		},
		{
			name:         "monthly uses calendar months",
			lastCharge:   time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC),
			billingCycle: "monthly",
			want:         "Renews 30 Aug",
		},
		{
			name:         "old daily opt-in advances many cycles without drift",
			lastCharge:   time.Date(2025, time.January, 1, 9, 30, 0, 0, time.UTC),
			billingCycle: "daily",
			want:         "Renews 25 Aug",
		},
		{
			name:         "unknown cycle yields no hint",
			lastCharge:   time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC),
			billingCycle: "quarterly",
			want:         "",
		},
		{
			name:         "empty cycle yields no hint",
			lastCharge:   time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC),
			billingCycle: "",
			want:         "",
		},
		{
			name:         "zero last charge yields no hint",
			lastCharge:   time.Time{},
			billingCycle: "daily",
			want:         "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := nextChargeHint(tc.lastCharge, tc.billingCycle, now); got != tc.want {
				t.Fatalf("nextChargeHint(%v, %q) = %q, want %q", tc.lastCharge, tc.billingCycle, got, tc.want)
			}
		})
	}
}
