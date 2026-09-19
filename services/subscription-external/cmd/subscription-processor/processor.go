package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
)

type jobStatus struct {
	ID         string `json:"id"`
	Processed  int    `json:"processed"`
	State      string `json:"state"`
	Total      int    `json:"total"`
	Successful int    `json:"successful"`
	Failed     int    `json:"failed"`
}

type processor struct {
	config Config
	client *http.Client
	secret string
	output io.Writer
}

func (p *processor) request(ctx context.Context, method, endpoint string, body []byte, result any) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	timestamp := time.Now().UTC().Format(time.RFC3339)
	mac := hmac.New(sha256.New, []byte(p.secret))
	_, _ = mac.Write(append([]byte(timestamp), body...))
	req.Header.Set("X-Internal-Timestamp", timestamp)
	req.Header.Set("X-Internal-Signature", hex.EncodeToString(mac.Sum(nil)))
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s request failed (check connectivity and request_timeout)", method)
	}
	defer resp.Body.Close()
	expected := http.StatusOK
	if method == http.MethodPost {
		expected = http.StatusAccepted
	}
	if resp.StatusCode != expected {
		return fmt.Errorf("%s returned HTTP %d", method, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return fmt.Errorf("read API response: %w", err)
	}
	if len(data) > 1<<20 {
		return fmt.Errorf("API response exceeds 1 MiB")
	}
	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("invalid API response")
	}
	return nil
}

func (p *processor) poll(ctx context.Context, id string) (jobStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, p.config.maxPoll)
	defer cancel()
	endpoint := p.config.BaseURL + "/api/v1/subscription-external/batch?jobId=" + url.QueryEscape(id)
	lastProgress := ""
	for {
		var status jobStatus
		if err := p.request(ctx, http.MethodGet, endpoint, nil, &status); err != nil {
			return status, err
		}
		if status.ID != id {
			return status, fmt.Errorf("status response job ID does not match checkpoint")
		}
		progress := fmt.Sprintf("Job %s: state=%s processed=%d/%d successful=%d failed=%d", id, status.State, status.Processed, status.Total, status.Successful, status.Failed)
		if progress != lastProgress {
			fmt.Fprintln(p.output, progress)
			lastProgress = progress
		}
		switch status.State {
		case "completed", "failed":
			return status, nil
		case "cancelled":
			return status, fmt.Errorf("job %s is %s; reconcile before starting another run", id, status.State)
		case "pending", "running":
		default:
			return status, fmt.Errorf("job returned an unknown state")
		}
		if err := wait(ctx, p.config.poll); err != nil {
			return status, err
		}
	}
}

func (p *processor) run(ctx context.Context, numbers []string, state *checkpoint) error {
	if len(numbers) == 0 {
		return fmt.Errorf("refusing an empty source")
	}
	if state.Submitting {
		return fmt.Errorf("previous submission outcome is unknown; reconcile it before editing or replacing the checkpoint")
	}
	endpoint := p.config.BaseURL + "/api/v1/subscription-external/batch"
	if state.Next < len(numbers) && state.JobID == "" {
		var capabilities struct {
			SubscriptionOnly     bool `json:"subscription_only"`
			InvalidMSISDNLogging bool `json:"invalid_msisdn_logging"`
		}
		if err := p.request(ctx, http.MethodGet, endpoint+"?capabilities=1&tenant_key="+url.QueryEscape(p.config.TenantKey), nil, &capabilities); err != nil {
			return fmt.Errorf("subscription-only preflight failed; nothing submitted: %w", err)
		}
		if !capabilities.SubscriptionOnly || !capabilities.InvalidMSISDNLogging {
			return fmt.Errorf("server does not support subscription-only processing and durable invalid-MSISDN logging; nothing submitted")
		}
	}
	for state.Next < len(numbers) {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := state.Next + min(p.config.BatchSize, len(numbers)-state.Next)
		if state.JobID == "" {
			request := domain.BatchOptinRequest{SubscriptionOnly: true, Count: end - state.Next, MSISDNS: numbers[state.Next:end],
				Telco: p.config.Telco, EntryChannel: p.config.EntryChannel, ProductIds: p.config.ProductIDs,
				TenantKey: p.config.TenantKey, ChannelKey: p.config.ChannelKey}
			body, err := json.Marshal(request)
			if err != nil {
				return err
			}
			// The API does not offer idempotent enqueue. Persist intent before the POST;
			// never automatically replay a request whose acceptance is uncertain.
			state.Submitting = true
			if err := saveCheckpoint(p.config.StateFile, state); err != nil {
				return err
			}
			var accepted struct {
				JobID string `json:"jobId"`
			}
			if err := p.request(ctx, http.MethodPost, endpoint, body, &accepted); err != nil {
				return fmt.Errorf("enqueue outcome requires reconciliation: %w", err)
			}
			if accepted.JobID == "" {
				return fmt.Errorf("enqueue response missing jobId; submission outcome is unknown")
			}
			state.JobID, state.Submitting = accepted.JobID, false
			if err := saveCheckpoint(p.config.StateFile, state); err != nil {
				return fmt.Errorf("accepted job %s but checkpoint failed: %w", accepted.JobID, err)
			}
			fmt.Fprintf(p.output, "Accepted job %s for records %d-%d\n", state.JobID, state.Next+1, end)
		}
		status, err := p.poll(ctx, state.JobID)
		if err != nil {
			return fmt.Errorf("job %s retained in checkpoint: %w", state.JobID, err)
		}
		count := end - state.Next
		if status.Total != count || status.Successful < 0 || status.Successful > count || status.Failed < 0 || status.Failed != count-status.Successful {
			return fmt.Errorf("job %s returned inconsistent totals; checkpoint retained", state.JobID)
		}
		fmt.Fprintf(p.output, "Completed job %s: successful=%d failed=%d\n", state.JobID, status.Successful, status.Failed)
		state.Next, state.JobID = end, ""
		state.Successful += status.Successful
		state.Failed += status.Failed
		if err := saveCheckpoint(p.config.StateFile, state); err != nil {
			return err
		}
		if end < len(numbers) {
			if err := wait(ctx, p.config.wait); err != nil {
				return err
			}
		}
	}
	fmt.Fprintf(p.output, "Finished: processed=%d successful=%d failed=%d\n", state.Next, state.Successful, state.Failed)
	if state.Failed > 0 {
		return fmt.Errorf("%d subscription attempts failed; completed batches will not be replayed", state.Failed)
	}
	return nil
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
