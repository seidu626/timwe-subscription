package planner

import (
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/cadence-engine/internal/domain"
)

func TestPlannedMessageText_AppendsLinkURLForLinkContent(t *testing.T) {
	linkURL := "https://careerify.example/app"
	got := plannedMessageText(&domain.ContentItem{
		MessageText: "Open your lesson",
		ContentKind: "LINK",
		LinkURL:     &linkURL,
	})
	if got == nil || *got != "Open your lesson https://careerify.example/app" {
		t.Fatalf("plannedMessageText = %#v", got)
	}
}

func TestPlannedMessageText_TextContentLeavesOutboxTextNull(t *testing.T) {
	got := plannedMessageText(&domain.ContentItem{
		MessageText: "Plain text",
		ContentKind: "TEXT",
	})
	if got != nil {
		t.Fatalf("plannedMessageText = %#v, want nil", got)
	}
}

func TestOutboxIdempotencyKey_DistinctPerOccurrenceSamePerClaim(t *testing.T) {
	tenant := "t1"
	channel := "c1"
	sub := &domain.Subscription{ID: 42, PartnerRoleID: 7}
	series := &domain.MessageSeries{ID: 3, ContentVersion: 1}
	slot := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	state := domain.DueState{SubscriptionID: 42, CursorSeq: 5, NextSendAt: slot}

	first := outboxIdempotencyKey(&tenant, &channel, sub, series, state)
	reclaimed := outboxIdempotencyKey(&tenant, &channel, sub, series, state)
	if first != reclaimed {
		t.Fatalf("same occurrence must dedupe: %q != %q", first, reclaimed)
	}

	// A FAILED occurrence reschedules the state to a later slot with the same
	// cursor; the retry must not collide with the terminal job's key.
	state.NextSendAt = slot.Add(24 * time.Hour)
	retry := outboxIdempotencyKey(&tenant, &channel, sub, series, state)
	if retry == first {
		t.Fatalf("rescheduled occurrence must get a fresh key, got %q twice", first)
	}
}
