package observability

import (
	"regexp"
	"strings"
)

const (
	UnknownLabel = "unknown"
	InvalidLabel = "invalid"
)

var (
	labelValuePattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,96}$`)
	unsafeLabelKeys   = map[string]struct{}{
		"authorization": {},
		"body":          {},
		"click_id":      {},
		"headers":       {},
		"message_text":  {},
		"msisdn":        {},
		"password":      {},
		"secret":        {},
		"token":         {},
	}
	allowedLabelKeys = map[string]struct{}{
		"tenant_id":  {},
		"channel_id": {},
		"worker":     {},
		"status":     {},
	}
)

type WorkerLabelSet struct {
	TenantID  string
	ChannelID string
	Worker    string
}

func WorkerLabels(tenantID, channelID *string, worker string) WorkerLabelSet {
	return WorkerLabelSet{
		TenantID:  SafeLabelValue(ptrValue(tenantID)),
		ChannelID: SafeLabelValue(ptrValue(channelID)),
		Worker:    SafeLabelValue(worker),
	}
}

func SafeLabelValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return UnknownLabel
	}
	if !labelValuePattern.MatchString(value) {
		return InvalidLabel
	}
	return value
}

func ValidateMetricLabels(labels map[string]string) error {
	for key, value := range labels {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		if _, blocked := unsafeLabelKeys[normalizedKey]; blocked {
			return ErrUnsafeLabel{Key: normalizedKey}
		}
		if _, ok := allowedLabelKeys[normalizedKey]; !ok {
			return ErrUnsafeLabel{Key: normalizedKey}
		}
		if SafeLabelValue(value) == InvalidLabel {
			return ErrUnsafeLabel{Key: normalizedKey}
		}
	}
	return nil
}

type ErrUnsafeLabel struct {
	Key string
}

func (e ErrUnsafeLabel) Error() string {
	return "unsafe observability label: " + e.Key
}

func ptrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
