package scheduler

import (
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/cadence-engine/internal/domain"
)

func TestFirstSendAtDailyBeforePreferred(t *testing.T) {
	loc, _ := time.LoadLocation("Africa/Accra")
	now := time.Date(2026, 1, 17, 7, 0, 0, 0, loc)

	rule := domain.ScheduleRule{
		RuleKind:      RuleDaily,
		PreferredTime: time.Date(2000, 1, 1, 8, 30, 0, 0, loc),
		SendStartTime: time.Date(2000, 1, 1, 8, 0, 0, 0, loc),
		SendEndTime:   time.Date(2000, 1, 1, 20, 0, 0, 0, loc),
		Timezone:      "Africa/Accra",
	}

	got, err := FirstSendAt(rule, now, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := time.Date(2026, 1, 17, 8, 30, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestFirstSendAtDailyAfterPreferred(t *testing.T) {
	loc, _ := time.LoadLocation("Africa/Accra")
	now := time.Date(2026, 1, 17, 10, 0, 0, 0, loc)

	rule := domain.ScheduleRule{
		RuleKind:      RuleDaily,
		PreferredTime: time.Date(2000, 1, 1, 9, 0, 0, 0, loc),
		SendStartTime: time.Date(2000, 1, 1, 8, 0, 0, 0, loc),
		SendEndTime:   time.Date(2000, 1, 1, 20, 0, 0, 0, loc),
		Timezone:      "Africa/Accra",
	}

	got, err := FirstSendAt(rule, now, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := time.Date(2026, 1, 18, 9, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestWeeklyNextAllowedDay(t *testing.T) {
	loc, _ := time.LoadLocation("Africa/Accra")
	now := time.Date(2026, 1, 13, 9, 0, 0, 0, loc) // Tuesday

	rule := domain.ScheduleRule{
		RuleKind:      RuleWeekly,
		PreferredTime: time.Date(2000, 1, 1, 9, 0, 0, 0, loc),
		DaysOfWeek:    1 | 4 | 16, // Mon/Wed/Fri
		SendStartTime: time.Date(2000, 1, 1, 8, 0, 0, 0, loc),
		SendEndTime:   time.Date(2000, 1, 1, 20, 0, 0, 0, loc),
		Timezone:      "Africa/Accra",
	}

	got, err := FirstSendAt(rule, now, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := time.Date(2026, 1, 14, 9, 0, 0, 0, loc) // Wednesday
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestEveryNDaysFromAnchor(t *testing.T) {
	loc, _ := time.LoadLocation("Africa/Accra")
	anchor := time.Date(2026, 1, 17, 0, 0, 0, 0, loc)
	now := time.Date(2026, 1, 20, 12, 0, 0, 0, loc)

	rule := domain.ScheduleRule{
		RuleKind:      RuleEveryNDays,
		PreferredTime: time.Date(2000, 1, 1, 9, 0, 0, 0, loc),
		NDays:         2,
		SendStartTime: time.Date(2000, 1, 1, 8, 0, 0, 0, loc),
		SendEndTime:   time.Date(2000, 1, 1, 20, 0, 0, 0, loc),
		Timezone:      "Africa/Accra",
	}

	got, err := FirstSendAt(rule, now, anchor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := time.Date(2026, 1, 21, 9, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestSendWindowClampsPreferredTime(t *testing.T) {
	loc, _ := time.LoadLocation("Africa/Accra")
	now := time.Date(2026, 1, 17, 6, 0, 0, 0, loc)

	rule := domain.ScheduleRule{
		RuleKind:      RuleDaily,
		PreferredTime: time.Date(2000, 1, 1, 7, 0, 0, 0, loc),
		SendStartTime: time.Date(2000, 1, 1, 8, 0, 0, 0, loc),
		SendEndTime:   time.Date(2000, 1, 1, 18, 0, 0, 0, loc),
		Timezone:      "Africa/Accra",
	}

	got, err := FirstSendAt(rule, now, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := time.Date(2026, 1, 17, 8, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestCatchupThrottleAdvancesToFuture(t *testing.T) {
	loc, _ := time.LoadLocation("Africa/Accra")
	now := time.Date(2026, 1, 17, 10, 0, 0, 0, loc)
	lastSent := time.Date(2026, 1, 10, 9, 0, 0, 0, loc)

	rule := domain.ScheduleRule{
		RuleKind:      RuleDaily,
		PreferredTime: time.Date(2000, 1, 1, 9, 0, 0, 0, loc),
		SendStartTime: time.Date(2000, 1, 1, 8, 0, 0, 0, loc),
		SendEndTime:   time.Date(2000, 1, 1, 20, 0, 0, 0, loc),
		Timezone:      "Africa/Accra",
		CatchupMode:   CatchupThrottle,
	}

	got, err := NextSendAt(rule, now, lastSent, lastSent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := time.Date(2026, 1, 18, 9, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}
