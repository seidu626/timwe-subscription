package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// readSource freezes and validates the whole feed before any subscription is submitted.
func readSource(ctx context.Context, c Config, client *http.Client, stdin io.Reader) ([]string, int, error) {
	var reader io.Reader
	var closer io.Closer
	switch {
	case c.Source == "database":
		return readDatabase(ctx, c)
	case c.Source == "-":
		reader = stdin
	case strings.HasPrefix(c.Source, "http://") || strings.HasPrefix(c.Source, "https://"):
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Source, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid source URL")
		}
		if req.URL.User != nil || req.URL.Fragment != "" {
			return nil, 0, fmt.Errorf("source URL cannot contain credentials or a fragment")
		}
		if c.SourceTokenEnv != "" {
			token := strings.TrimSpace(os.Getenv(c.SourceTokenEnv))
			if token == "" {
				return nil, 0, fmt.Errorf("source token environment variable is empty")
			}
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("source request failed (check connectivity and request_timeout)")
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, 0, fmt.Errorf("source returned HTTP %d", resp.StatusCode)
		}
		reader = resp.Body
	default:
		file, err := os.Open(c.Source)
		if err != nil {
			return nil, 0, fmt.Errorf("open source: %w", err)
		}
		reader, closer = file, file
	}
	if closer != nil {
		defer closer.Close()
	}
	var data []byte
	var err error
	if c.Source == "-" {
		// stdin may be a pipe whose writer never closes. Let runCLI unwind its
		// checkpoint lock on cancellation; process exit terminates the reader.
		type result struct {
			data []byte
			err  error
		}
		read := make(chan result, 1)
		go func() {
			data, err := io.ReadAll(io.LimitReader(reader, c.MaxSourceBytes+1))
			read <- result{data, err}
		}()
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case value := <-read:
			data, err = value.data, value.err
		}
	} else {
		data, err = io.ReadAll(io.LimitReader(reader, c.MaxSourceBytes+1))
	}
	if err != nil {
		return nil, 0, fmt.Errorf("read source: %w", err)
	}
	if int64(len(data)) > c.MaxSourceBytes {
		return nil, 0, fmt.Errorf("source exceeds max_source_bytes")
	}
	return parseSource(data, c.Format)
}

func parseSource(data []byte, format string) ([]string, int, error) {
	data = bytes.TrimSpace(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
	if len(data) == 0 {
		return nil, 0, fmt.Errorf("source contains no MSISDNs")
	}
	if format == "auto" {
		format = "csv"
		if data[0] == '[' || data[0] == '{' {
			format = "json"
		}
	}
	var raw []string
	switch format {
	case "json":
		var err error
		if data[0] == '{' {
			var envelope struct {
				MSISDNs []string `json:"msisdns"`
			}
			err = json.Unmarshal(data, &envelope)
			raw = envelope.MSISDNs
		} else {
			err = json.Unmarshal(data, &raw)
		}
		if err != nil {
			return nil, 0, fmt.Errorf("invalid JSON feed: use a string array or an object with a msisdns string array")
		}
	case "text":
		raw = strings.Split(string(data), "\n")
	case "csv":
		r := csv.NewReader(bytes.NewReader(data))
		r.TrimLeadingSpace = true
		rows, err := r.ReadAll()
		if err != nil {
			return nil, 0, fmt.Errorf("invalid CSV feed")
		}
		if len(rows) == 0 {
			return nil, 0, fmt.Errorf("source contains no MSISDNs")
		}
		column, start := -1, 0
		for i, value := range rows[0] {
			if strings.EqualFold(strings.TrimSpace(value), "msisdn") {
				if column >= 0 {
					return nil, 0, fmt.Errorf("CSV has multiple msisdn columns")
				}
				column, start = i, 1
			}
		}
		if column < 0 {
			if len(rows[0]) != 1 {
				return nil, 0, fmt.Errorf("multi-column CSV requires a msisdn header")
			}
			column = 0
		}
		for _, row := range rows[start:] {
			raw = append(raw, row[column])
		}
	default:
		return nil, 0, fmt.Errorf("unsupported source format")
	}
	seen := make(map[string]bool, len(raw))
	numbers := make([]string, 0, len(raw))
	duplicates := 0
	for i, value := range raw {
		number := strings.TrimSpace(value)
		if number == "" {
			if format == "text" {
				continue
			}
			return nil, 0, fmt.Errorf("empty MSISDN at record %d", i+1)
		}
		number = strings.TrimPrefix(number, "+")
		if len(number) < 7 || len(number) > 15 {
			return nil, 0, fmt.Errorf("invalid MSISDN at record %d: expected 7-15 digits", i+1)
		}
		for _, digit := range number {
			if digit < '0' || digit > '9' {
				return nil, 0, fmt.Errorf("invalid MSISDN at record %d: expected digits only", i+1)
			}
		}
		if seen[number] {
			duplicates++
			continue
		}
		seen[number] = true
		numbers = append(numbers, number)
	}
	if len(numbers) == 0 {
		return nil, 0, fmt.Errorf("source contains no MSISDNs")
	}
	return numbers, duplicates, nil
}
