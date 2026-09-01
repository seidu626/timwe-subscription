package handler

import (
	"context"
	"sync"
	"testing"

	"github.com/seidu626/subscription-manager/common/config"
	msisdncatalog "github.com/seidu626/subscription-manager/common/msisdn/catalog"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"go.uber.org/zap"
)

type catalogTestGenerator struct {
	mu      sync.Mutex
	batches [][]string
}

func (g *catalogTestGenerator) GenerateBatchMSISDNSOptimized(context.Context, string, int, *config.Config) ([]string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.batches) == 0 {
		return []string{}, nil
	}
	out := g.batches[0]
	g.batches = g.batches[1:]
	return out, nil
}
func (*catalogTestGenerator) GetDetailedStats() map[string]interface{} { return nil }

type catalogTestStore struct {
	mu       sync.Mutex
	nextID   int64
	existing map[string]bool
	events   []string
	verdicts map[int64]string
}

func (s *catalogTestStore) InsertGenerated(_ context.Context, _, _, _, _ string, msisdns []string) ([]msisdncatalog.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, "insert")
	var out []msisdncatalog.Record
	for _, msisdn := range msisdns {
		if s.existing[msisdn] {
			continue
		}
		s.existing[msisdn] = true
		s.nextID++
		out = append(out, msisdncatalog.Record{ID: s.nextID, MSISDN: msisdn})
	}
	return out, nil
}
func (s *catalogTestStore) SaveVerification(_ context.Context, _ string, id int64, result msisdncatalog.VerificationResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, "save")
	s.verdicts[id] = result.Status
	return nil
}

type catalogTestVerifier struct{ store *catalogTestStore }

func (v catalogTestVerifier) Verify(_ context.Context, _ string) msisdncatalog.VerificationResult {
	v.store.mu.Lock()
	defer v.store.mu.Unlock()
	foundInsert := false
	for _, event := range v.store.events {
		if event == "insert" {
			foundInsert = true
			break
		}
	}
	if !foundInsert {
		panic("verification happened before catalog insert")
	}
	return msisdncatalog.VerificationResult{Status: msisdncatalog.StatusVerified, Provider: "MADAPI"}
}

type catalogTestTenantResolver struct{}

func (catalogTestTenantResolver) TenantIDByKey(string) (string, error) { return "tenant-id", nil }

func TestGenerateVerifiedCatalogMSISDNsPersistsBeforeVerificationAndSkipsExisting(t *testing.T) {
	store := &catalogTestStore{existing: map[string]bool{"233551111111": true}, verdicts: map[int64]string{}}
	h := &SubscriptionHandler{
		logger:          zap.NewNop(),
		config:          &config.Config{},
		msisdnGenerator: &catalogTestGenerator{batches: [][]string{{"233551111111", "233552222222"}, {"233553333333"}}},
		msisdnCatalog:   store,
		msisdnVerifier:  catalogTestVerifier{store: store},
		tenantResolver:  catalogTestTenantResolver{},
	}
	got, err := h.generateVerifiedCatalogMSISDNs(context.Background(), "job-1", &domain.BatchOptinRequest{Count: 2, Telco: "MTN", TenantKey: "tenant"})
	if err != nil {
		t.Fatalf("generateVerifiedCatalogMSISDNs() error = %v", err)
	}
	if len(got) != 2 || got[0] != "233552222222" || got[1] != "233553333333" {
		t.Fatalf("got %#v", got)
	}
	if len(store.verdicts) != 2 {
		t.Fatalf("saved verdicts = %d, want 2", len(store.verdicts))
	}
}

func TestGenerateVerifiedCatalogMSISDNsFailsClosedWithoutCatalog(t *testing.T) {
	h := &SubscriptionHandler{}
	_, err := h.generateVerifiedCatalogMSISDNs(context.Background(), "job-1", &domain.BatchOptinRequest{Count: 1, Telco: "MTN"})
	if err == nil {
		t.Fatal("expected configuration error")
	}
}
