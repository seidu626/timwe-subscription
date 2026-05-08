package domain

import "testing"

func TestMapChargeToNotificationPreservesTenantChannel(t *testing.T) {
	got := MapChargeToNotification(ChargeRequest{
		ProductID:    14397,
		PricepointID: 1,
		MCC:          "620",
		MNC:          "03",
		MSISDN:       "233241234567",
		ShortCode:    "1234",
		Context:      "renewal",
		Channel:      "SMS",
		TenantRoute: TenantRouteContext{
			TenantID:  "tenant-1",
			ChannelID: "channel-1",
		},
	}, 9090)

	if got.TenantID == nil || *got.TenantID != "tenant-1" {
		t.Fatalf("expected tenant on charge notification, got %#v", got.TenantID)
	}
	if got.ChannelID == nil || *got.ChannelID != "channel-1" {
		t.Fatalf("expected channel on charge notification, got %#v", got.ChannelID)
	}
	if got.Type != "CHARGE" || got.TransactionUUID == "" {
		t.Fatalf("expected charge notification with transaction uuid, got %+v", got)
	}
}

func TestMapChargeToNotificationInvalidBlankIdempotencyFallsBack(t *testing.T) {
	got := MapChargeToNotification(ChargeRequest{
		ProductID:      14397,
		PricepointID:   1,
		MSISDN:         "233241234567",
		IdempotencyKey: "   ",
	}, 9090)

	if got.TransactionUUID == "" {
		t.Fatal("expected fallback transaction uuid for invalid blank idempotency key")
	}
	if got.ExternalTxID != got.TransactionUUID {
		t.Fatalf("expected external tx id to match ownership uuid, got %+v", got)
	}
}
