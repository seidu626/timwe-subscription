package handler

import (
	"testing"
)

func TestValidateTrackingConfig(t *testing.T) {
	t.Run("accepts empty or null", func(t *testing.T) {
		if err := validateTrackingConfig(nil); err != nil {
			t.Fatalf("expected nil error for empty config, got %v", err)
		}
		if err := validateTrackingConfig([]byte("null")); err != nil {
			t.Fatalf("expected nil error for null config, got %v", err)
		}
	})

	t.Run("accepts valid schema", func(t *testing.T) {
		raw := []byte(`{
			"pixels": {
				"facebook": {"pixel_id": "fb-123", "enabled": true},
				"google": {"measurement_id": "G-123", "ads_id": "AW-123", "enabled": false},
				"tiktok": {"pixel_id": "tt-456", "enabled": true}
			},
			"attribution": {"model": "last_touch", "window_days": 7},
			"custom_events": [{"name": "signup", "trigger": "submit"}]
		}`)
		if err := validateTrackingConfig(raw); err != nil {
			t.Fatalf("expected nil error for valid config, got %v", err)
		}
	})

	t.Run("rejects unknown fields", func(t *testing.T) {
		raw := []byte(`{
			"pixels": {
				"facebook": {"pixel_id": "fb-123", "enabled": true, "extra": "nope"}
			}
		}`)
		if err := validateTrackingConfig(raw); err == nil {
			t.Fatal("expected error for unknown fields")
		}
	})

	t.Run("rejects missing required fields", func(t *testing.T) {
		raw := []byte(`{
			"pixels": {
				"facebook": {"pixel_id": "fb-123"}
			}
		}`)
		if err := validateTrackingConfig(raw); err == nil {
			t.Fatal("expected error for missing required fields")
		}
	})

	t.Run("rejects invalid attribution", func(t *testing.T) {
		raw := []byte(`{
			"attribution": {"model": "unknown", "window_days": 7}
		}`)
		if err := validateTrackingConfig(raw); err == nil {
			t.Fatal("expected error for invalid attribution model")
		}
	})
}
