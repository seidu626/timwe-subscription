package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestJSONSeverityIsMachineReadable(t *testing.T) {
	for _, rolling := range []bool{false, true} {
		name := "regular"
		if rolling {
			name = "rolling"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "events.log")
			var logger *zap.Logger
			var err error
			if rolling {
				logger, err = NewRollingFileLogger(path, RollingLogConfig{Enabled: true, MaxSize: 1, MaxBackups: 1})
			} else {
				logger, err = NewZapLogger(path)
			}
			if err != nil {
				t.Fatal(err)
			}
			logger.Info("accepted")
			logger.Warn("retrying")
			logger.Error("persistence failed")
			_ = logger.Sync()
			files, err := filepath.Glob(filepath.Join(dir, "*.log"))
			if err != nil || len(files) != 1 {
				t.Fatalf("expected one log file: %v %v", files, err)
			}
			data, err := os.ReadFile(files[0])
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) != 3 {
				t.Fatalf("expected 3 records, got %d", len(lines))
			}
			for i, want := range []string{"info", "warn", "error"} {
				var record map[string]interface{}
				if err := json.Unmarshal([]byte(lines[i]), &record); err != nil {
					t.Fatal(err)
				}
				if record["level"] != want {
					t.Fatalf("record %d severity %q, expected %q", i, record["level"], want)
				}
			}
		})
	}
}
