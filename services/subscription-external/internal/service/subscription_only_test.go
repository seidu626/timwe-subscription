package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type subscriptionOnlyRepo struct {
	MockSubscriptionRepository
	saved         []*domain.SubscriptionRequest
	invalid       []*domain.InvalidMSISDNLog
	logError      error
	invalidErrors []error
	invalidCalls  int
	createErrors  []error
	createCalls   int
}

func (r *subscriptionOnlyRepo) CreateSubscription(req *domain.SubscriptionRequest) error {
	r.createCalls++
	if len(r.createErrors) > 0 {
		err := r.createErrors[0]
		r.createErrors = r.createErrors[1:]
		if err != nil {
			return err
		}
	}
	r.saved = append(r.saved, req)
	return nil
}
func (r *subscriptionOnlyRepo) CreateInvalidMSISDNLog(entry *domain.InvalidMSISDNLog) error {
	r.invalidCalls++
	r.invalid = append(r.invalid, entry)
	if len(r.invalidErrors) > 0 {
		err := r.invalidErrors[0]
		r.invalidErrors = r.invalidErrors[1:]
		return err
	}
	return r.logError
}
func (r *subscriptionOnlyRepo) CheckSubscriptionExists(string, int) (bool, error) {
	panic("subscription-only must not enter renewal handling")
}
func (r *subscriptionOnlyRepo) CreateNotification(*domain.NotificationRequest) error {
	panic("subscription-only must not send notifications")
}

type knownInvalidUserBase struct{ MockUserBaseRepository }

func (*knownInvalidUserBase) IsExcludedUserForTenant(context.Context, string, string) (bool, error) {
	return false, nil
}

func (*knownInvalidUserBase) GetInvalidMSISDNSFast(context.Context, string) (bool, error) {
	return true, nil
}

type failingBlacklistUserBase struct {
	MockUserBaseRepository
	calls int
}

type failingEligibilityUserBase struct {
	MockUserBaseRepository
	exclusionErr error
	invalidErr   error
}

func (r *failingEligibilityUserBase) IsExcludedUserForTenant(context.Context, string, string) (bool, error) {
	return false, r.exclusionErr
}

func (r *failingEligibilityUserBase) GetInvalidMSISDNSFast(context.Context, string) (bool, error) {
	return false, r.invalidErr
}

func (r *failingBlacklistUserBase) UpsertBlacklistedUser(context.Context, string, string) error {
	r.calls++
	return &pq.Error{Code: "53300"}
}

func TestSubscriptionOnlyOptinNoSMSAndInvalidPersistence(t *testing.T) {
	for _, scenario := range []string{"already-active", "waiting-charging", "invalid", "invalid-no-data", "invalid-storage-fails", "no-WAP-config", "needs-confirmation"} {
		t.Run(scenario, func(t *testing.T) {
			const providerEchoSecret = "provider-echo-233241234567"
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
				var data interface{} = map[string]interface{}{"subscriptionResult": result, "transactionId": "provider-tx", "providerEcho": providerEchoSecret}
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
			var logs bytes.Buffer
			svc.logger = zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&logs), zap.DebugLevel))
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
				if scenario == "invalid-storage-fails" {
					var localEvidenceErr *SubscriptionOnlyLocalEvidenceError
					if !errors.As(err, &localEvidenceErr) || localEvidenceErr.ProviderCode != SubscriptionResultInvalidMsisdn ||
						localEvidenceErr.LocalPersistenceStage != "invalid_msisdn_registry" || localEvidenceErr.Attempts != 1 {
						t.Fatalf("storage failure evidence incomplete: %T %+v", err, localEvidenceErr)
					}
				}
			}
			if strings.Contains(logs.String(), providerEchoSecret) {
				t.Fatalf("subscription-only logs leaked arbitrary provider response data: %s", logs.String())
			}
		})
	}
}

func TestSubscriptionOnlyKnownInvalidNeverCallsProvider(t *testing.T) {
	svc := newSubscriptionServiceForExternalTxIDTest("http://unused.invalid")
	svc.UserBaseRepository = &knownInvalidUserBase{}
	configureContractTenantProvider(svc, "http://unused.invalid")
	err := svc.ProcessOptin(&domain.OptinRequest{SubscriptionOnly: true, Msisdn: "233270000001", EntryChannel: "WAP", TenantRoute: contractTenantRoute()})
	var result *domain.MTResponseError
	if !errors.As(err, &result) || result.Code != "INVALID_MSISDN" {
		t.Fatalf("expected known-invalid rejection, got %v", err)
	}
}

func TestSubscriptionOnlyEligibilityReadFailurePreventsProviderAndRetainsSafeEvidence(t *testing.T) {
	for _, tt := range []struct {
		name         string
		stage        string
		exclusionErr error
		invalidErr   error
	}{
		{name: "tenant exclusion registry", stage: "tenant_exclusion_read", exclusionErr: &pq.Error{Code: "53300"}},
		{name: "invalid MSISDN registry", stage: "invalid_msisdn_registry_read", invalidErr: &pq.Error{Code: "53300"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			providerCalls := 0
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				providerCalls++
			}))
			defer server.Close()

			svc := newSubscriptionServiceForExternalTxIDTest(server.URL)
			svc.UserBaseRepository = &failingEligibilityUserBase{exclusionErr: tt.exclusionErr, invalidErr: tt.invalidErr}
			configureContractTenantProvider(svc, server.URL)
			err := svc.ProcessOptin(&domain.OptinRequest{
				SubscriptionOnly: true,
				Msisdn:           "233270000001",
				EntryChannel:     "WAP",
				ProductIds:       []string{"32535"},
				TenantRoute:      contractTenantRoute(),
			})

			var localErr *SubscriptionOnlyLocalEvidenceError
			if !errors.As(err, &localErr) {
				t.Fatalf("error = %T %v, want local preflight read failure", err, err)
			}
			if providerCalls != 0 || localErr.Outcome != "provider_not_attempted" || localErr.LocalPersistenceStage != tt.stage ||
				localErr.PostgresCode != "53300" || !localErr.Transient || localErr.Attempts != 1 ||
				localErr.TenantID != "tenant-contract" || localErr.ChannelID != "channel-contract" ||
				localErr.ProviderRequestID != "" || localErr.ExternalTransactionID != "" || localErr.TrackingID != "" {
				t.Fatalf("preflight failure evidence incomplete: %+v", localErr.SafeDetails())
			}
		})
	}
}

func TestSubscriptionOnlyPersistenceRetryNeverRepeatsProviderOptin(t *testing.T) {
	tests := []struct {
		name             string
		createErrors     []error
		wantCreateCalls  int
		wantPostgresCode string
		wantErr          bool
	}{
		{
			name:            "transient 53300 then success",
			createErrors:    []error{&pq.Error{Code: "53300"}, nil},
			wantCreateCalls: 2,
		},
		{
			name:             "permanent constraint failure",
			createErrors:     []error{&pq.Error{Code: "23514"}},
			wantCreateCalls:  1,
			wantPostgresCode: "23514",
			wantErr:          true,
		},
		{
			name: "transient failure exhausted",
			createErrors: []error{
				&pq.Error{Code: "53300"},
				&pq.Error{Code: "53300"},
				&pq.Error{Code: "53300"},
			},
			wantCreateCalls:  3,
			wantPostgresCode: "53300",
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerCalls := 0
			requestTrackingID := ""
			requestExternalTxID := ""
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				providerCalls++
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				requestTrackingID, _ = body["trackingId"].(string)
				requestExternalTxID = r.Header.Get("external-tx-id")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code":      "SUCCESS",
					"requestId": "provider-request",
					"responseData": map[string]interface{}{
						"subscriptionResult": "OPTIN_ALREADY_ACTIVE",
						"transactionId":      "provider-transaction",
					},
				})
			}))
			defer server.Close()

			repo := &subscriptionOnlyRepo{createErrors: append([]error(nil), tt.createErrors...)}
			svc := newSubscriptionServiceForExternalTxIDTest(server.URL)
			svc.repo = repo
			svc.circuitBreaker = gobreaker.NewTwoStepCircuitBreaker(gobreaker.Settings{})
			svc.bulkhead = make(chan struct{}, 1)
			configureContractTenantProvider(svc, server.URL)

			rawMSISDN := "233270000001"
			startedAt := time.Now().UTC()
			err := svc.processOptinForProduct(
				&domain.OptinRequest{SubscriptionOnly: true, Msisdn: rawMSISDN, EntryChannel: "WAP", TenantRoute: contractTenantRoute()},
				&domain.Product{ProductId: "32535", Name: "Careerify", PricePointId: 70946},
			)
			finishedAt := time.Now().UTC()

			if providerCalls != 1 {
				t.Fatalf("provider opt-in calls = %d, want exactly 1", providerCalls)
			}
			if repo.createCalls != tt.wantCreateCalls {
				t.Fatalf("CreateSubscription calls = %d, want %d", repo.createCalls, tt.wantCreateCalls)
			}
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("unexpected persistence error: %v", err)
				}
				if len(repo.saved) != 1 {
					t.Fatalf("saved subscriptions = %d, want 1", len(repo.saved))
				}
				if repo.saved[0].TrackingId == nil || *repo.saved[0].TrackingId != requestTrackingID || requestTrackingID == "" {
					t.Fatalf("persisted tracking ID does not match exact provider request: saved=%v request=%q", repo.saved[0].TrackingId, requestTrackingID)
				}
				return
			}

			var persistenceErr *AcceptedSubscriptionPersistenceError
			if !errors.As(err, &persistenceErr) {
				t.Fatalf("error = %T %v, want AcceptedSubscriptionPersistenceError", err, err)
			}
			if persistenceErr.PostgresCode != tt.wantPostgresCode || persistenceErr.ProviderRequestID != "provider-request" ||
				persistenceErr.ProviderTransactionID != "provider-transaction" || persistenceErr.TenantID != "tenant-contract" ||
				persistenceErr.TrackingID != requestTrackingID || persistenceErr.ExternalTransactionID != requestExternalTxID ||
				requestTrackingID == "" || requestExternalTxID == "" || requestTrackingID == requestExternalTxID {
				t.Fatalf("unsafe or incomplete persistence receipt: %+v", persistenceErr.SafeDetails())
			}
			if persistenceErr.ProviderAcceptedAt.Location() != time.UTC || persistenceErr.ProviderAcceptedAt.Before(startedAt) || persistenceErr.ProviderAcceptedAt.After(finishedAt) {
				t.Fatalf("provider acceptance time = %s, want UTC within call [%s, %s]", persistenceErr.ProviderAcceptedAt, startedAt, finishedAt)
			}
			detailsJSON, marshalErr := json.Marshal(persistenceErr.SafeDetails())
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if strings.Contains(err.Error(), rawMSISDN) || strings.Contains(string(detailsJSON), rawMSISDN) {
				t.Fatal("persistence failure evidence leaked raw MSISDN")
			}
		})
	}
}

func TestSubscriptionOnlyProviderFailuresAttemptOnceWithSafeCorrelation(t *testing.T) {
	for _, scenario := range []string{"network-close", "http-500", "invalid-json", "success-missing-result", "success-unsupported-result"} {
		t.Run(scenario, func(t *testing.T) {
			const providerBodySecret = "provider-body-233241234567"
			var capturedMu sync.Mutex
			providerCalls := 0
			requestTrackingID := ""
			requestExternalTxID := ""
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedMu.Lock()
				defer capturedMu.Unlock()
				providerCalls++
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				requestTrackingID, _ = body["trackingId"].(string)
				requestExternalTxID = r.Header.Get("external-tx-id")
				if body["entryChannel"] != "WAP" {
					t.Errorf("unexpected fallback channel: %v", body["entryChannel"])
				}

				switch scenario {
				case "network-close":
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					_ = conn.Close()
				case "http-500":
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(providerBodySecret))
				case "invalid-json":
					_, _ = w.Write([]byte("{"))
				case "success-missing-result":
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"code": "SUCCESS", "requestId": "provider-request", "responseData": map[string]interface{}{},
					})
				case "success-unsupported-result":
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"code": "SUCCESS", "requestId": "provider-request", "responseData": map[string]interface{}{"subscriptionResult": "OPTIN_PREACTIVE_WAIT_CONF"},
					})
				}
			}))
			defer server.Close()

			repo := &subscriptionOnlyRepo{}
			svc := newSubscriptionServiceForExternalTxIDTest(server.URL)
			var logs bytes.Buffer
			svc.logger = zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&logs), zap.DebugLevel))
			svc.repo = repo
			svc.circuitBreaker = gobreaker.NewTwoStepCircuitBreaker(gobreaker.Settings{})
			svc.bulkhead = make(chan struct{}, 1)
			configureContractTenantProvider(svc, server.URL)

			err := svc.processOptinForProduct(
				&domain.OptinRequest{SubscriptionOnly: true, Msisdn: "233270000001", EntryChannel: "WAP", TenantRoute: contractTenantRoute()},
				&domain.Product{ProductId: "32535", Name: "Careerify", PricePointId: 70946},
			)
			capturedMu.Lock()
			capturedCalls := providerCalls
			capturedTrackingID := requestTrackingID
			capturedExternalTxID := requestExternalTxID
			capturedMu.Unlock()
			if capturedCalls != 1 {
				t.Fatalf("provider POST calls = %d, want exactly 1", capturedCalls)
			}
			if repo.createCalls != 0 {
				t.Fatalf("local persistence calls = %d, want 0 for unknown provider outcome", repo.createCalls)
			}

			var outcomeErr *SubscriptionOnlyProviderOutcomeUnknownError
			if !errors.As(err, &outcomeErr) {
				t.Fatalf("error = %T %v, want SubscriptionOnlyProviderOutcomeUnknownError", err, err)
			}
			if outcomeErr.ExternalTransactionID != capturedExternalTxID || outcomeErr.TrackingID != capturedTrackingID ||
				capturedExternalTxID == "" || capturedTrackingID == "" || outcomeErr.TenantID != "tenant-contract" ||
				capturedExternalTxID == capturedTrackingID || outcomeErr.ChannelID != "channel-contract" || outcomeErr.ProductID != 32535 {
				t.Fatalf("incomplete outcome-unknown correlation: %+v", outcomeErr.SafeDetails())
			}
			if details := outcomeErr.SafeDetails(); details["acceptedByProvider"] != false || details["outcome"] != "outcome_unknown" {
				t.Fatalf("incorrect outcome classification: %+v", details)
			}
			if strings.Contains(logs.String(), providerBodySecret) {
				t.Fatalf("subscription-only logs leaked raw non-200 provider body: %s", logs.String())
			}
		})
	}
}

func TestSubscriptionOnlyAcceptedMissingTransactionRetainsRecoveryEvidence(t *testing.T) {
	providerCalls := 0
	requestTrackingID := ""
	requestExternalTxID := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerCalls++
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		requestTrackingID, _ = body["trackingId"].(string)
		requestExternalTxID = r.Header.Get("external-tx-id")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "SUCCESS", "requestId": "provider-request",
			"responseData": map[string]interface{}{"subscriptionResult": "OPTIN_ALREADY_ACTIVE"},
		})
	}))
	defer server.Close()

	repo := &subscriptionOnlyRepo{}
	svc := newSubscriptionServiceForExternalTxIDTest(server.URL)
	svc.repo = repo
	svc.circuitBreaker = gobreaker.NewTwoStepCircuitBreaker(gobreaker.Settings{})
	svc.bulkhead = make(chan struct{}, 1)
	configureContractTenantProvider(svc, server.URL)
	startedAt := time.Now().UTC()
	err := svc.processOptinForProduct(
		&domain.OptinRequest{SubscriptionOnly: true, Msisdn: "233270000001", EntryChannel: "WAP", TenantRoute: contractTenantRoute()},
		&domain.Product{ProductId: "32535", Name: "Careerify", PricePointId: 70946},
	)
	finishedAt := time.Now().UTC()

	var acceptedErr *AcceptedSubscriptionPersistenceError
	if !errors.As(err, &acceptedErr) {
		t.Fatalf("error = %T %v, want accepted materialization failure", err, err)
	}
	if providerCalls != 1 || repo.createCalls != 0 || acceptedErr.FailureStage != "provider_transaction_id" ||
		acceptedErr.ProviderRequestID != "provider-request" || acceptedErr.ProviderTransactionID != "" ||
		acceptedErr.ExternalTransactionID != requestExternalTxID || acceptedErr.TrackingID != requestTrackingID ||
		requestExternalTxID == "" || requestTrackingID == "" || requestExternalTxID == requestTrackingID {
		t.Fatalf("accepted materialization evidence incomplete: %+v", acceptedErr.SafeDetails())
	}
	if acceptedErr.ProviderAcceptedAt.Before(startedAt) || acceptedErr.ProviderAcceptedAt.After(finishedAt) || acceptedErr.ProviderAcceptedAt.Location() != time.UTC {
		t.Fatalf("invalid accepted timestamp: %s", acceptedErr.ProviderAcceptedAt)
	}
}

func TestSubscriptionOnlyBlacklistedRegistryFailureIsSynchronousAndRecoverable(t *testing.T) {
	providerCalls := 0
	requestTrackingID := ""
	requestExternalTxID := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerCalls++
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		requestTrackingID, _ = body["trackingId"].(string)
		requestExternalTxID = r.Header.Get("external-tx-id")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "BLACKLISTED", "requestId": "provider-request",
			"responseData": map[string]interface{}{"subscriptionResult": "BLACKLISTED"},
		})
	}))
	defer server.Close()

	userBase := &failingBlacklistUserBase{}
	repo := &subscriptionOnlyRepo{}
	svc := newSubscriptionServiceForExternalTxIDTest(server.URL)
	svc.repo = repo
	svc.UserBaseRepository = userBase
	svc.circuitBreaker = gobreaker.NewTwoStepCircuitBreaker(gobreaker.Settings{})
	svc.bulkhead = make(chan struct{}, 1)
	configureContractTenantProvider(svc, server.URL)
	err := svc.processOptinForProduct(
		&domain.OptinRequest{SubscriptionOnly: true, Msisdn: "233270000001", EntryChannel: "WAP", TenantRoute: contractTenantRoute()},
		&domain.Product{ProductId: "32535", Name: "Careerify", PricePointId: 70946},
	)

	var localErr *SubscriptionOnlyLocalEvidenceError
	if !errors.As(err, &localErr) {
		t.Fatalf("error = %T %v, want local evidence failure", err, err)
	}
	if providerCalls != 1 || userBase.calls != 3 || localErr.ProviderCode != ResponseCodeBlacklisted ||
		localErr.LocalPersistenceStage != "blacklist_registry" || localErr.Attempts != 3 || localErr.PostgresCode != "53300" ||
		localErr.ProviderRequestID != "provider-request" || localErr.ExternalTransactionID != requestExternalTxID ||
		localErr.TrackingID != requestTrackingID || requestExternalTxID == "" || requestTrackingID == "" {
		t.Fatalf("blacklist local persistence evidence incomplete: %+v", localErr.SafeDetails())
	}
	callsAtReturn := userBase.calls
	time.Sleep(50 * time.Millisecond)
	if userBase.calls != callsAtReturn {
		t.Fatalf("blacklist persistence continued asynchronously after return: before=%d after=%d", callsAtReturn, userBase.calls)
	}
}

func TestSubscriptionOnlyInvalidRegistryTransientFailureRetainsRecoveryEvidence(t *testing.T) {
	providerCalls := 0
	requestTrackingID := ""
	requestExternalTxID := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerCalls++
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		requestTrackingID, _ = body["trackingId"].(string)
		requestExternalTxID = r.Header.Get("external-tx-id")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "INVALID_MSISDN", "requestId": "provider-request",
			"responseData": map[string]interface{}{"subscriptionResult": "INVALID_MSISDN"},
		})
	}))
	defer server.Close()

	repo := &subscriptionOnlyRepo{invalidErrors: []error{
		&pq.Error{Code: "53300"}, &pq.Error{Code: "53300"}, &pq.Error{Code: "53300"},
	}}
	svc := newSubscriptionServiceForExternalTxIDTest(server.URL)
	svc.repo = repo
	svc.circuitBreaker = gobreaker.NewTwoStepCircuitBreaker(gobreaker.Settings{})
	svc.bulkhead = make(chan struct{}, 1)
	configureContractTenantProvider(svc, server.URL)
	err := svc.processOptinForProduct(
		&domain.OptinRequest{SubscriptionOnly: true, Msisdn: "233270000001", EntryChannel: "WAP", TenantRoute: contractTenantRoute()},
		&domain.Product{ProductId: "32535", Name: "Careerify", PricePointId: 70946},
	)

	var localErr *SubscriptionOnlyLocalEvidenceError
	if !errors.As(err, &localErr) {
		t.Fatalf("error = %T %v, want local evidence failure", err, err)
	}
	if providerCalls != 1 || repo.invalidCalls != 3 || localErr.ProviderCode != SubscriptionResultInvalidMsisdn ||
		localErr.LocalPersistenceStage != "invalid_msisdn_registry" || localErr.Attempts != 3 || localErr.PostgresCode != "53300" ||
		localErr.ProviderRequestID != "provider-request" || localErr.ExternalTransactionID != requestExternalTxID ||
		localErr.TrackingID != requestTrackingID || requestExternalTxID == "" || requestTrackingID == "" {
		t.Fatalf("invalid registry evidence incomplete: %+v", localErr.SafeDetails())
	}
}
