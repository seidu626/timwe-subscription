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
			*dest[1].(*int) = 2117
			*dest[2].(*string) = "233241234567"
			*dest[3].(*int) = 8509
			*dest[4].(*sql.NullString) = sql.NullString{String: "SMS", Valid: true}
			*dest[5].(*int) = 7
			*dest[6].(*sql.NullString) = sql.NullString{String: "MO", Valid: true}
			*dest[7].(*time.Time) = createdAt
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
			*dest[1].(*int) = 1
			*dest[2].(*string) = "233200000000"
			*dest[3].(*int) = 1
			*dest[5].(*int) = 1
			*dest[7].(*time.Time) = createdAt
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
