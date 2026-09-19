package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/seidu626/subscription-manager/common/config"
)

func TestJSONErrorSeverityIsMachineReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.log")
	logger, err := NewZapLogger(path, &config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	logger.Error("persistence failed")
	_ = logger.Sync()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]interface{}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record["level"] != "error" {
		t.Fatalf("machine severity must equal error, got %q", record["level"])
	}
}
