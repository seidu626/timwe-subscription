package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/sony/gobreaker"
)

type subscriptionOnlyRepo struct {
	MockSubscriptionRepository
	saved    []*domain.SubscriptionRequest
	invalid  []*domain.InvalidMSISDNLog
	logError error
}

func (r *subscriptionOnlyRepo) CreateSubscription(req *domain.SubscriptionRequest) error {
	r.saved = append(r.saved, req)
	return nil
}
func (r *subscriptionOnlyRepo) CreateInvalidMSISDNLog(entry *domain.InvalidMSISDNLog) error {
	r.invalid = append(r.invalid, entry)
	return r.logError
}
func (r *subscriptionOnlyRepo) CheckSubscriptionExists(string, int) (bool, error) {
	panic("subscription-only must not enter renewal handling")
}
func (r *subscriptionOnlyRepo) CreateNotification(*domain.NotificationRequest) error {
	panic("subscription-only must not send notifications")
}

type knownInvalidUserBase struct{ MockUserBaseRepository }

func (*knownInvalidUserBase) GetInvalidMSISDNSFast(context.Context, string) (bool, error) {
	return true, nil
}

func TestSubscriptionOnlyOptinNoSMSAndInvalidPersistence(t *testing.T) {
	for _, scenario := range []string{"already-active", "waiting-charging", "invalid", "invalid-no-data", "invalid-storage-fails", "no-WAP-config", "needs-confirmation"} {
		t.Run(scenario, func(t *testing.T) {
			requests := 0
			requestExternalID := ""
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.URL.Path != "/subscription/optin/2117" {
					t.Errorf("unexpected side-effect endpoint %s", r.URL.Path)
				}
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["entryChannel"] != "WAP" {
					t.Error("SMS fallback attempted")
				}
				requestExternalID = r.Header.Get("external-tx-id")
				code, result := "SUCCESS", "OPTIN_ALREADY_ACTIVE"
				switch scenario {
				case "waiting-charging":
					result = "OPTIN_ACTIVE_WAIT_CHARGING"
				case "invalid", "invalid-no-data", "invalid-storage-fails":
					code, result = "INVALID_MSISDN", "INVALID_MSISDN"
				case "no-WAP-config":
					result = "OPTIN_CONFIG_NOT_FOUND"
				case "needs-confirmation":
					result = "OPTIN_PREACTIVE_WAIT_CONF"
				}
				var data interface{} = map[string]interface{}{"subscriptionResult": result, "transactionId": "provider-tx"}
				if scenario == "invalid-no-data" {
					data = nil
				}
				json.NewEncoder(w).Encode(map[string]interface{}{"code": code, "requestId": "provider-request", "responseData": data})
			}))
			defer server.Close()
			repo := &subscriptionOnlyRepo{}
			if scenario == "invalid-storage-fails" {
				repo.logError = errors.New("database unavailable")
			}
			svc := newSubscriptionServiceForExternalTxIDTest(server.URL)
			svc.repo = repo
			svc.circuitBreaker = gobreaker.NewTwoStepCircuitBreaker(gobreaker.Settings{})
			svc.bulkhead = make(chan struct{}, 1)
			configureContractTenantProvider(svc, server.URL)
			err := svc.processOptinForProduct(&domain.OptinRequest{SubscriptionOnly: true, Msisdn: "233270000001", EntryChannel: "WAP", TenantRoute: contractTenantRoute()}, &domain.Product{ProductId: "32535", Name: "Careerify", PricePointId: 70946})
			if requests != 1 {
				t.Fatalf("expected exactly one provider opt-in, got %d", requests)
			}
			if scenario == "already-active" || scenario == "waiting-charging" {
				if err != nil {
					t.Fatal(err)
				}
				if len(repo.saved) != 1 {
					t.Fatal("subscription not persisted")
				}
				saved := repo.saved[0]
				if saved.TenantID == nil || *saved.TenantID != "tenant-contract" || saved.ChannelID == nil || *saved.ChannelID != "channel-contract" {
					t.Fatal("tenant route lost")
				}
				if saved.PartnerRoleId != 2117 || saved.TransactionId != "provider-tx" {
					t.Fatal("provider identity lost")
				}
			} else {
				if err == nil {
					t.Fatal("nonterminal/rejected result reported as success")
				}
				if len(repo.saved) != 0 {
					t.Fatal("rejected/pending confirmation result persisted as active")
				}
			}
			if strings.HasPrefix(scenario, "invalid") {
				if len(repo.invalid) != 1 {
					t.Fatal("invalid number not logged synchronously")
				}
				if repo.invalid[0].ExternalTxID != requestExternalID || repo.invalid[0].RequestID != "provider-request" || repo.invalid[0].MSISDN != "233270000001" {
					t.Fatal("invalid evidence correlation lost")
				}
				if scenario == "invalid-storage-fails" && !strings.Contains(err.Error(), "persist INVALID_MSISDN evidence") {
					t.Fatalf("storage failure hidden: %v", err)
				}
			}
		})
	}
}

func TestSubscriptionOnlyKnownInvalidNeverCallsProvider(t *testing.T) {
	svc := newSubscriptionServiceForExternalTxIDTest("http://unused.invalid")
	svc.UserBaseRepository = &knownInvalidUserBase{}
	err := svc.ProcessOptin(&domain.OptinRequest{SubscriptionOnly: true, Msisdn: "233270000001", EntryChannel: "WAP"})
	var result *domain.MTResponseError
	if !errors.As(err, &result) || result.Code != "INVALID_MSISDN" {
		t.Fatalf("expected known-invalid rejection, got %v", err)
	}
}
