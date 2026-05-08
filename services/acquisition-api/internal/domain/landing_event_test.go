package domain

import (
	"testing"
)

func TestLandingEventType_IsValid(t *testing.T) {
	testCases := []struct {
		eventType LandingEventType
		expected  bool
	}{
		{EventLandingView, true},
		{EventLandingClick, true},
		{EventFormSubmit, true},
		{LandingEventType("invalid"), false},
		{LandingEventType(""), false},
		{LandingEventType("LANDING_VIEW"), false}, // case sensitive
	}

	for _, tc := range testCases {
		t.Run(string(tc.eventType), func(t *testing.T) {
			result := tc.eventType.IsValid()
			if result != tc.expected {
				t.Errorf("IsValid() for %q = %v, want %v", tc.eventType, result, tc.expected)
			}
		})
	}
}

func TestCreateLandingEventRequest_Validate(t *testing.T) {
	tests := []struct {
		name      string
		request   CreateLandingEventRequest
		wantError bool
		errorField string
	}{
		{
			name: "valid request",
			request: CreateLandingEventRequest{
				EventType:    EventLandingView,
				CampaignSlug: "gh-tigo-daily",
			},
			wantError: false,
		},
		{
			name: "valid request with click ID",
			request: CreateLandingEventRequest{
				EventType:    EventLandingClick,
				CampaignSlug: "gh-tigo-daily",
				ClickID:      strPtr("abc123"),
				AdProvider:   strPtr("mobplus"),
			},
			wantError: false,
		},
		{
			name: "missing campaign slug",
			request: CreateLandingEventRequest{
				EventType: EventLandingView,
			},
			wantError:  true,
			errorField: "campaign_slug",
		},
		{
			name: "invalid event type",
			request: CreateLandingEventRequest{
				EventType:    LandingEventType("invalid"),
				CampaignSlug: "gh-tigo-daily",
			},
			wantError:  true,
			errorField: "event_type",
		},
		{
			name: "empty event type",
			request: CreateLandingEventRequest{
				CampaignSlug: "gh-tigo-daily",
			},
			wantError:  true,
			errorField: "event_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.wantError {
				if err == nil {
					t.Error("expected validation error, got nil")
					return
				}
				if ve, ok := err.(*ValidationError); ok {
					if tt.errorField != "" && ve.Field != tt.errorField {
						t.Errorf("expected error field %q, got %q", tt.errorField, ve.Field)
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected validation error: %v", err)
				}
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
