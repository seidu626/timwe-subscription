package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	_ "github.com/lib/pq"
)

// readDatabase takes one PostgreSQL snapshot. The operator supplies a stable
// ORDER BY in the query so the same rows produce the same checkpoint fingerprint.
func readDatabase(ctx context.Context, c Config) ([]string, int, error) {
	dsn := strings.TrimSpace(os.Getenv(c.DatabaseURLEnv))
	if dsn == "" {
		return nil, 0, fmt.Errorf("database URL environment variable is empty")
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid PostgreSQL configuration")
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, 0, fmt.Errorf("cannot begin read-only database snapshot (check connection and request_timeout)")
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, c.DatabaseQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("database source query failed; query must be read-only")
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil || len(columns) != 1 || !strings.EqualFold(columns[0], "msisdn") {
		return nil, 0, fmt.Errorf("database query must return exactly one column named msisdn")
	}
	var values []string
	var size int64 = 2 // JSON array delimiters
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, 0, fmt.Errorf("database MSISDN must be a non-null string")
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, 0, fmt.Errorf("cannot encode database MSISDN")
		}
		size += int64(len(encoded)) + 1
		if size > c.MaxSourceBytes {
			return nil, 0, fmt.Errorf("database source exceeds max_source_bytes")
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("database source read failed")
	}
	if rows.NextResultSet() {
		return nil, 0, fmt.Errorf("database query must return only one result set")
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("database source read failed")
	}
	data, err := json.Marshal(values)
	if err != nil {
		return nil, 0, err
	}
	return parseSource(data, "json")
}
