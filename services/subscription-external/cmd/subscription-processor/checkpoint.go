package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type checkpoint struct {
	Version     int    `json:"version"`
	Fingerprint string `json:"fingerprint"`
	Next        int    `json:"next"`
	JobID       string `json:"job_id,omitempty"`
	Submitting  bool   `json:"submitting"`
	Successful  int    `json:"successful"`
	Failed      int    `json:"failed"`
}

func loadCheckpoint(c Config, numbers []string) (*checkpoint, error) {
	// Include every value that changes which subscriptions a resumed batch creates.
	identity := struct {
		BaseURL, Tenant, Channel, Telco, Entry string
		Products, Numbers                      []string
		BatchSize                              int
	}{c.BaseURL, c.TenantKey, c.ChannelKey, c.Telco, c.EntryChannel, c.ProductIDs, numbers, c.BatchSize}
	data, err := json.Marshal(identity)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	fingerprint := hex.EncodeToString(sum[:])
	state := &checkpoint{Version: 1, Fingerprint: fingerprint}
	data, err = os.ReadFile(c.StateFile)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return nil, err
	}
	state = &checkpoint{}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("invalid checkpoint: %w", err)
	}
	if state.Version != 1 || state.Fingerprint != fingerprint {
		return nil, fmt.Errorf("checkpoint does not match source or subscription route; use the original input or a separate state_file for a new run")
	}
	if state.Next < 0 || state.Next > len(numbers) || state.Successful < 0 || state.Successful > state.Next || state.Failed < 0 || state.Failed != state.Next-state.Successful ||
		(state.Next != len(numbers) && state.Next%c.BatchSize != 0) ||
		(state.Submitting && state.JobID != "") || (state.Next == len(numbers) && (state.JobID != "" || state.Submitting)) {
		return nil, fmt.Errorf("checkpoint contains inconsistent progress")
	}
	return state, nil
}

func saveCheckpoint(path string, state *checkpoint) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".subscription-progress-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return err
	}
	// Persist the rename as well as the contents before issuing a subscription POST.
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func lockCheckpoint(path string) (func(), error) {
	lockPath := path + ".lock"
	file, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("cannot lock checkpoint (another process or stale .lock file): %w", err)
	}
	if _, err := fmt.Fprintf(file, "%d\n", os.Getpid()); err != nil {
		file.Close()
		os.Remove(lockPath)
		return nil, err
	}
	if err := file.Close(); err != nil {
		os.Remove(lockPath)
		return nil, err
	}
	return func() { _ = os.Remove(lockPath) }, nil
}
