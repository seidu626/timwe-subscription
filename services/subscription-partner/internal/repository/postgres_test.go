package repository

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeRowScanner struct {
	scanFn func(dest ...interface{}) error
}

func (f fakeRowScanner) Scan(dest ...interface{}) error {
	return f.scanFn(dest...)
}

func TestScanAndMapSubscription_MapsValues(t *testing.T) {
	start := time.Date(2026, time.February, 12, 10, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	created := start.Add(-1 * time.Hour)

	scanner := fakeRowScanner{
		scanFn: func(dest ...interface{}) error {
			*dest[0].(*int) = 11
			*dest[1].(*string) = "233241234567"
			*dest[2].(*string) = "MSISDN"
			*dest[3].(*int) = 8509
			*dest[4].(*int) = 2117
			*dest[5].(*sql.NullString) = sql.NullString{String: "https://campaign.example", Valid: true}
			*dest[6].(*sql.NullString) = sql.NullString{String: "SMS", Valid: true}
			*dest[7].(*sql.NullString) = sql.NullString{String: "SUB", Valid: true}
			*dest[8].(*sql.NullString) = sql.NullString{String: "trk-123", Valid: true}
			*dest[9].(*sql.NullString) = sql.NullString{String: "acct-1", Valid: true}
			*dest[10].(*sql.NullString) = sql.NullString{String: "620", Valid: true}
			*dest[11].(*sql.NullString) = sql.NullString{String: "01", Valid: true}
			*dest[12].(*sql.NullString) = sql.NullString{String: "active", Valid: true}
			*dest[13].(*sql.NullInt64) = sql.NullInt64{Int64: 3, Valid: true}
			*dest[14].(*sql.NullInt64) = sql.NullInt64{Int64: 7, Valid: true}
			*dest[15].(*time.Time) = start
			*dest[16].(*sql.NullTime) = sql.NullTime{Time: end, Valid: true}
			*dest[17].(*sql.NullString) = sql.NullString{String: "auth-777", Valid: true}
			*dest[18].(*sql.NullString) = sql.NullString{String: "127.0.0.1", Valid: true}
			*dest[19].(*time.Time) = created
			return nil
		},
	}

	sub, err := scanAndMapSubscription(scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.Id != 11 || sub.PartnerRoleId != "2117" || sub.ProductId != "8509" {
		t.Fatalf("unexpected identity fields: %+v", sub)
	}
	if sub.CancelReason == nil || *sub.CancelReason != "3" {
		t.Fatalf("expected cancelReason=3, got %+v", sub.CancelReason)
	}
	if sub.CancelSource == nil || *sub.CancelSource != "7" {
		t.Fatalf("expected cancelSource=7, got %+v", sub.CancelSource)
	}
	if sub.EndDate == nil || !sub.EndDate.Equal(end) {
		t.Fatalf("unexpected end date: %+v", sub.EndDate)
	}
	if sub.TransactionAuthCode == nil || *sub.TransactionAuthCode != "auth-777" {
		t.Fatalf("unexpected transaction auth code: %+v", sub.TransactionAuthCode)
	}
	if sub.CreatedAt != created.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected createdAt: %s", sub.CreatedAt)
	}
}

func TestScanAndMapSubscription_HandlesNullablesAndErrors(t *testing.T) {
	start := time.Date(2026, time.February, 12, 10, 0, 0, 0, time.UTC)

	nullScanner := fakeRowScanner{
		scanFn: func(dest ...interface{}) error {
			*dest[0].(*int) = 1
			*dest[1].(*string) = "233200000000"
			*dest[2].(*string) = "MSISDN"
			*dest[3].(*int) = 1
			*dest[4].(*int) = 1
			*dest[15].(*time.Time) = start
			*dest[19].(*time.Time) = start
			return nil
		},
	}

	sub, err := scanAndMapSubscription(nullScanner)
	if err != nil {
		t.Fatalf("unexpected error for null scanner: %v", err)
	}
	if sub.CancelReason != nil || sub.CancelSource != nil || sub.EndDate != nil || sub.TransactionAuthCode != nil {
		t.Fatalf("expected nullable fields to remain nil: %+v", sub)
	}

	expected := errors.New("scan failed")
	errScanner := fakeRowScanner{
		scanFn: func(dest ...interface{}) error {
			return expected
		},
	}

	_, err = scanAndMapSubscription(errScanner)
	if !errors.Is(err, expected) {
		t.Fatalf("expected error propagation, got: %v", err)
	}
}

func TestApplySubscriptionDateFilters_UsesCreatedAtWindow(t *testing.T) {
	startDate := time.Date(2025, time.December, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, time.December, 1, 23, 59, 59, 0, time.UTC)
	query := "SELECT * FROM subscriptions WHERE 1=1"
	countQuery := "SELECT COUNT(*) FROM subscriptions WHERE 1=1"

	filteredQuery, filteredCountQuery, args, nextArg := applySubscriptionDateFilters(query, countQuery, nil, 1, startDate, endDate)

	if !strings.Contains(filteredQuery, "AND created_at >= $1") || !strings.Contains(filteredQuery, "AND created_at <= $2") {
		t.Fatalf("expected created_at range in query, got: %s", filteredQuery)
	}
	if strings.Contains(filteredQuery, "start_date") || strings.Contains(filteredQuery, "end_date") {
		t.Fatalf("did not expect lifecycle date columns in query, got: %s", filteredQuery)
	}
	if !strings.Contains(filteredCountQuery, "AND created_at >= $1") || !strings.Contains(filteredCountQuery, "AND created_at <= $2") {
		t.Fatalf("expected created_at range in count query, got: %s", filteredCountQuery)
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 filter args, got %d", len(args))
	}
	if got, ok := args[0].(time.Time); !ok || !got.Equal(startDate) {
		t.Fatalf("unexpected startDate arg: %#v", args[0])
	}
	if got, ok := args[1].(time.Time); !ok || !got.Equal(endDate) {
		t.Fatalf("unexpected endDate arg: %#v", args[1])
	}
	if nextArg != 3 {
		t.Fatalf("expected next arg index 3, got %d", nextArg)
	}
}
