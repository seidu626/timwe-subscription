package planner

import (
	"testing"

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
