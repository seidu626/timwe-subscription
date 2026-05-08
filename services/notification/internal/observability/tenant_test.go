package observability

import "testing"

func TestValidateMetricLabelsRejectsPIIAndSecrets(t *testing.T) {
	if err := ValidateMetricLabels(map[string]string{"msisdn": "233241234567"}); err == nil {
		t.Fatal("expected error for msisdn label")
	}
	if err := ValidateMetricLabels(map[string]string{"click_id": "abc-123"}); err == nil {
		t.Fatal("expected error for click_id label")
	}
	if err := ValidateMetricLabels(map[string]string{"tenant_id": "tenant-a", "channel_id": "sms", "worker": "notification_worker"}); err != nil {
		t.Fatalf("expected approved labels, got %v", err)
	}
}

func TestWorkerLabelsAreBoundedAndSafe(t *testing.T) {
	tenant := "tenant-a"
	channel := "sms"
	labels := WorkerLabels(&tenant, &channel, "notification_worker")
	if labels.TenantID != "tenant-a" || labels.ChannelID != "sms" || labels.Worker != "notification_worker" {
		t.Fatalf("unexpected labels: %#v", labels)
	}
	unsafe := "tenant with spaces and punctuation!!!"
	labels = WorkerLabels(&unsafe, nil, "notification_worker")
	if labels.TenantID != InvalidLabel || labels.ChannelID != UnknownLabel {
		t.Fatalf("expected invalid/unknown labels, got %#v", labels)
	}
}
