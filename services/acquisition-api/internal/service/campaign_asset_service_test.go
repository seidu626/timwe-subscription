package service

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeStorageEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "keeps host", input: "s3.amazonaws.com", expected: "s3.amazonaws.com"},
		{name: "strips scheme", input: "https://storage.example.com", expected: "storage.example.com"},
		{name: "strips trailing slash", input: "storage.example.com/", expected: "storage.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeStorageEndpoint(tt.input); got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestSanitizeAssetSegment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "normal", input: "gh-airteltigo", expected: "gh-airteltigo"},
		{name: "replaces spaces", input: "GH Airtel Tigo", expected: "gh-airtel-tigo"},
		{name: "removes punctuation", input: "gh@airtel!", expected: "gh-airtel"},
		{name: "trims dashes", input: "__gh__", expected: "gh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeAssetSegment(tt.input); got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestBuildBackgroundObjectKeyIncludesTenantNamespace(t *testing.T) {
	got := buildBackgroundObjectKey("campaign-backgrounds", "tenant-a", "gh-airteltigo", time.Unix(1710000000, 0), "asset-id", ".png")
	expected := "campaign-backgrounds/tenants/tenant-a/gh-airteltigo/1710000000-asset-id.png"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestBuildBackgroundObjectKeySeparatesTenantNamespaces(t *testing.T) {
	now := time.Unix(1710000000, 0)
	tenantA := buildBackgroundObjectKey("campaign-backgrounds", "tenant-a", "shared-campaign", now, "asset-id", ".png")
	tenantB := buildBackgroundObjectKey("campaign-backgrounds", "tenant-b", "shared-campaign", now, "asset-id", ".png")

	if tenantA == tenantB {
		t.Fatalf("expected tenant namespaces to produce distinct object keys, got %q", tenantA)
	}
	if !strings.Contains(tenantA, "/tenants/tenant-a/") || !strings.Contains(tenantB, "/tenants/tenant-b/") {
		t.Fatalf("expected tenant namespaces in keys, got %q and %q", tenantA, tenantB)
	}
}

func TestValidateAssetFileNameRejectsTraversalAndControlCharacters(t *testing.T) {
	bad := []string{"../bg.png", `nested\bg.png`, "safe/unsafe.png", "bad\x00name.png"}
	for _, input := range bad {
		if err := validateAssetFileName(input); err == nil {
			t.Fatalf("expected error for %q", input)
		}
	}
	if err := validateAssetFileName("background.png"); err != nil {
		t.Fatalf("expected valid file name, got %v", err)
	}
}

func TestBuildAssetURLUsesPublicBase(t *testing.T) {
	svc := &CampaignAssetService{cfg: CampaignAssetStorageConfig{PublicBaseURL: "https://cdn.example.com/assets", Bucket: "campaign", UseSSL: true, Endpoint: "s3.example.com"}}
	got := svc.buildAssetURL("campaign-backgrounds/gh/test.png")
	expected := "https://cdn.example.com/assets/campaign-backgrounds/gh/test.png"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestBuildAssetURLFallsBackToBucketPath(t *testing.T) {
	svc := &CampaignAssetService{cfg: CampaignAssetStorageConfig{Bucket: "campaign", UseSSL: true, Endpoint: "https://s3.example.com"}}
	got := svc.buildAssetURL("campaign-backgrounds/gh/test.png")
	expected := "https://s3.example.com/campaign/campaign-backgrounds/gh/test.png"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
