package handler

import (
	"strings"
	"testing"
)

func TestParseDNDCSVAcceptsHeaderAndFirstColumn(t *testing.T) {
	rows, err := parseDNDCSV(strings.NewReader("msisdn,note\n233551111111,requested\n233552222222,requested\n"))
	if err != nil {
		t.Fatalf("parseDNDCSV() error = %v", err)
	}
	if len(rows) != 2 || rows[0] != "233551111111" || rows[1] != "233552222222" {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestParseDNDCSVAcceptsHeaderlessFile(t *testing.T) {
	rows, err := parseDNDCSV(strings.NewReader("233551111111\n"))
	if err != nil {
		t.Fatalf("parseDNDCSV() error = %v", err)
	}
	if len(rows) != 1 || rows[0] != "233551111111" {
		t.Fatalf("rows = %#v", rows)
	}
}
