package repository

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

type fakeRowScanner struct {
	scanFn func(dest ...interface{}) error
}

func (f fakeRowScanner) Scan(dest ...interface{}) error {
	return f.scanFn(dest...)
}

func TestScanAndMapNotification_MapsValues(t *testing.T) {
	createdAt := time.Date(2026, time.February, 12, 22, 0, 0, 0, time.UTC)

	scanner := fakeRowScanner{
		scanFn: func(dest ...interface{}) error {
			*dest[0].(*int) = 42
			*dest[1].(*sql.NullString) = sql.NullString{String: "tenant-1", Valid: true}
			*dest[2].(*sql.NullString) = sql.NullString{String: "channel-1", Valid: true}
			*dest[3].(*int) = 2117
			*dest[4].(*string) = "233241234567"
			*dest[5].(*int) = 8509
			*dest[6].(*sql.NullString) = sql.NullString{String: "SMS", Valid: true}
			*dest[7].(*int) = 7
			*dest[8].(*sql.NullString) = sql.NullString{String: "MO", Valid: true}
			*dest[9].(*time.Time) = createdAt
			return nil
		},
	}

	notification, err := scanAndMapNotification(scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if notification.ID != 42 || notification.PartnerRole != 2117 || notification.ProductID != 8509 {
		t.Fatalf("unexpected core fields: %+v", notification)
	}
	if notification.TenantID == nil || *notification.TenantID != "tenant-1" {
		t.Fatalf("unexpected tenant id: %#v", notification.TenantID)
	}
	if notification.ChannelID == nil || *notification.ChannelID != "channel-1" {
		t.Fatalf("unexpected channel id: %#v", notification.ChannelID)
	}
	if notification.EntryChannel != "SMS" {
		t.Fatalf("unexpected entry channel: %q", notification.EntryChannel)
	}
	if notification.Type == nil || *notification.Type != "MO" {
		t.Fatalf("unexpected type pointer: %+v", notification.Type)
	}
	if !notification.CreatedAt.Equal(createdAt) {
		t.Fatalf("unexpected createdAt: %s", notification.CreatedAt)
	}
}

func TestScanAndMapNotification_NullablesAndErrorPropagation(t *testing.T) {
	createdAt := time.Date(2026, time.February, 12, 22, 0, 0, 0, time.UTC)
	scanner := fakeRowScanner{
		scanFn: func(dest ...interface{}) error {
			*dest[0].(*int) = 1
			*dest[3].(*int) = 1
			*dest[4].(*string) = "233200000000"
			*dest[5].(*int) = 1
			*dest[7].(*int) = 1
			*dest[9].(*time.Time) = createdAt
			return nil
		},
	}

	notification, err := scanAndMapNotification(scanner)
	if err != nil {
		t.Fatalf("unexpected error for nullable scan: %v", err)
	}
	if notification.Type != nil {
		t.Fatalf("expected nil type, got: %+v", notification.Type)
	}
	if notification.EntryChannel != "" {
		t.Fatalf("expected empty entry channel, got: %q", notification.EntryChannel)
	}

	rootErr := errors.New("scan failed")
	errScanner := fakeRowScanner{
		scanFn: func(dest ...interface{}) error {
			return rootErr
		},
	}

	_, err = scanAndMapNotification(errScanner)
	if !errors.Is(err, rootErr) {
		t.Fatalf("expected wrapped root error, got: %v", err)
	}
}

func TestGenerateCacheKeySeparatesTenantChannel(t *testing.T) {
	repo := &NotificationRepository{}
	start := time.Date(2026, time.May, 8, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	first := repo.GenerateCacheKey(start, end, "tenant-1", "channel-1", "2117", "", "", "MO", 1, 10)
	second := repo.GenerateCacheKey(start, end, "tenant-2", "channel-1", "2117", "", "", "MO", 1, 10)
	third := repo.GenerateCacheKey(start, end, "tenant-1", "channel-2", "2117", "", "", "MO", 1, 10)

	if first == second || first == third || second == third {
		t.Fatalf("expected tenant/channel-specific cache keys, got %q %q %q", first, second, third)
	}
}
