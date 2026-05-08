package repository

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/lib/pq"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
)

func TestBuildCreateChargeNotificationOnceArgsPreservesTenantChannel(t *testing.T) {
	tenantID := "tenant-1"
	channelID := "channel-1"
	notification := &domain.NotificationRequest{
		TenantID:        &tenantID,
		ChannelID:       &channelID,
		PartnerRole:     9090,
		ExternalTxID:    "idem-1",
		ProductID:       14397,
		PricepointID:    1,
		MCC:             "620",
		MNC:             "03",
		MSISDN:          "233241234567",
		LargeAccount:    "1234",
		TransactionUUID: "idem-1",
		EntryChannel:    "SMS",
		MessageType:     "Charge",
		Message:         "renewal",
		Tags:            []string{"billing", "charge"},
		Type:            "CHARGE",
	}

	args := buildCreateChargeNotificationOnceArgs(notification)

	if args[0] != (sql.NullString{String: "tenant-1", Valid: true}) {
		t.Fatalf("expected tenant sql value, got %#v", args[0])
	}
	if args[1] != (sql.NullString{String: "channel-1", Valid: true}) {
		t.Fatalf("expected channel sql value, got %#v", args[1])
	}
	if args[10] != "idem-1" || args[16] != "CHARGE" {
		t.Fatalf("expected idempotent charge args, got %#v", args)
	}
}

func TestIsUniqueViolationRecognizesPostgresDuplicateKey(t *testing.T) {
	if !isUniqueViolation(&pq.Error{Code: "23505"}) {
		t.Fatal("expected duplicate key pq error to be recognized")
	}
	if isUniqueViolation(errors.New("ordinary failure")) {
		t.Fatal("ordinary errors must not be treated as idempotent duplicates")
	}
}
