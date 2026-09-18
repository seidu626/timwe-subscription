package main

import (
	"context"
	"database/sql"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

// This test requires a disposable database. It never uses application configuration.
func TestPostgresSourceIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_SUBSCRIPTION_PROCESSOR_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_SUBSCRIPTION_PROCESSOR_DATABASE_URL is not set; PostgreSQL behavior not verified")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var database string
	if err := db.QueryRow("SELECT current_database()").Scan(&database); err != nil {
		t.Fatal(err)
	}
	if database != "subscription_processor_test" {
		t.Fatalf("refusing non-test database %q", database)
	}
	c := testConfig(t)
	c.Source, c.DatabaseURLEnv = "database", "TEST_SUBSCRIPTION_PROCESSOR_DATABASE_URL"
	c.DatabaseQuery = "SELECT number AS msisdn FROM (VALUES ('233240000002'), ('+233240000001'), ('233240000002')) AS feed(number) ORDER BY number"
	numbers, duplicates, err := readSource(context.Background(), c, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(numbers, []string{"233240000001", "233240000002"}) || duplicates != 1 {
		t.Fatalf("numbers=%v duplicates=%d", numbers, duplicates)
	}
	if err := runCLI(context.Background(), []string{"-config", writeConfig(t, c), "-dry-run"}, nil, io.Discard); err != nil {
		t.Fatal(err)
	}
	c.DatabaseQuery = "SELECT CASE WHEN current_setting('transaction_read_only') = 'on' AND current_setting('transaction_isolation') = 'repeatable read' THEN '233240000001' ELSE 'wrong isolation' END AS msisdn"
	if _, _, err := readSource(context.Background(), c, nil, nil); err != nil {
		t.Fatalf("database transaction settings: %v", err)
	}
	c.DatabaseQuery = "CREATE TABLE subscription_processor_forbidden (id integer)"
	if _, _, err := readSource(context.Background(), c, nil, nil); err == nil || !strings.Contains(err.Error(), "query must be read-only") {
		t.Fatalf("write was not rejected at query execution: %v", err)
	}
	for _, query := range []string{
		"CREATE TABLE subscription_processor_forbidden (id integer)",
		"SELECT NULL::text AS msisdn",
		"SELECT '233240000001' AS wrong_column",
		"SELECT '233240000001' AS msisdn, 'extra' AS extra",
		"SELECT 'invalid' AS msisdn",
		"SELECT '233240000001' AS msisdn WHERE false",
		"SELECT '233240000001' AS msisdn; SELECT '233240000002' AS msisdn",
	} {
		c.DatabaseQuery = query
		if _, _, err := readSource(context.Background(), c, nil, nil); err == nil {
			t.Fatalf("invalid query accepted: %s", query)
		}
	}
	var absent bool
	if err := db.QueryRow("SELECT to_regclass('subscription_processor_forbidden') IS NULL").Scan(&absent); err != nil || !absent {
		t.Fatalf("read-only query created a table: absent=%v err=%v", absent, err)
	}
	c.DatabaseQuery = "SELECT repeat('1', 100) AS msisdn"
	c.MaxSourceBytes = 20
	if _, _, err := readSource(context.Background(), c, nil, nil); err == nil {
		t.Fatal("oversized database source accepted")
	}
}
