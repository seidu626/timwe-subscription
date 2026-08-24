// slice-harness: allow-new-canonical-path: partner-optin-persistence tests
// tenant-scoped subscription persistence on gateway-trust partner
// optin/confirm/optout success.
package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
	"go.uber.org/zap"
)

// --- stubs ---

// stubTenantPersistRepo implements both gatewayTenantLookup and
// subscriptionPersister so it can be installed via WithTenantRepo and
// exercised through the type-assertion path persistPartner* uses.
type stubTenantPersistRepo struct {
	*stubGatewayRepo

	createErr   error
	updateErr   error
	createCalls []domain.SubscriptionRequest
	updateCalls []stubUpdateCall
}

type stubExistingOptinNotifier struct {
	err   error
	calls []stubExistingOptinNotificationCall
}

type stubExistingOptinNotificationCall struct {
	route        domain.TenantRouteContext
	partnerRole  int
	notification domain.NotificationRequest
}

func (n *stubExistingOptinNotifier) NotifyUserOptin(_ context.Context, route domain.TenantRouteContext, partnerRole int, notification *domain.NotificationRequest) error {
	n.calls = append(n.calls, stubExistingOptinNotificationCall{
		route:        route,
		partnerRole:  partnerRole,
		notification: *notification,
	})
	return n.err
}

type stubUpdateCall struct {
	msisdn    string
	productID string
	status    string
}

func (r *stubTenantPersistRepo) CreateSubscription(request *domain.SubscriptionRequest) error {
	r.createCalls = append(r.createCalls, *request)
	return r.createErr
}

func (r *stubTenantPersistRepo) UpdateSubscriptionStatus(msisdn string, productID string, status string) error {
	r.updateCalls = append(r.updateCalls, stubUpdateCall{msisdn: msisdn, productID: productID, status: status})
	return r.updateErr
}

func newStubTenantPersistRepo() *stubTenantPersistRepo {
	return &stubTenantPersistRepo{stubGatewayRepo: stubRepo()}
}

// stubTenantProviderResolver implements service.TenantProviderResolver so
// SubscriptionService.PartnerRoleIDForRoute can be exercised without a
// database, per existing test conventions (no live DB in unit tests).
type stubTenantProviderResolver struct {
	cfg *service.TenantProviderConfig
	err error
}

func (r *stubTenantProviderResolver) Resolve(_ context.Context, _ service.ChannelOperation, _ domain.TenantRouteContext) (*service.TenantProviderConfig, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.cfg, nil
}

// newPersistenceTestHandler builds a PartnerHandler with a resolvable
// PartnerRoleIDForRoute (via a stub TenantProviderResolver) and the given
// tenant repo installed for the persistence type assertion.
func newPersistenceTestHandler(repo gatewayTenantLookup, partnerRoleID string, resolveErr error) *PartnerHandler {
	svc := &service.SubscriptionService{}
	svc.SetTenantProviderRouter(&stubTenantProviderResolver{
		cfg: &service.TenantProviderConfig{PartnerRoleID: partnerRoleID},
		err: resolveErr,
	})
	h := NewPartnerHandler(zap.NewNop(), svc, &config.Config{})
	if repo != nil {
		h.WithTenantRepo(repo)
	}
	return h
}

func testRoute() domain.TenantRouteContext {
	return domain.TenantRouteContext{
		TenantID:   "tenant-uuid-1",
		TenantKey:  "careerify",
		ChannelID:  "channel-uuid-1",
		ChannelKey: "web-gh-airteltigo",
	}
}

func mtResponseWith(subscriptionResult, transactionID string) *domain.MTResponse {
	data := map[string]interface{}{}
	if subscriptionResult != "" {
		data["subscriptionResult"] = subscriptionResult
	}
	if transactionID != "" {
		data["transactionId"] = transactionID
	}
	return &domain.MTResponse{ResponseData: data, Code: "SUCCESS"}
}

// --- pure helper function tests ---

func TestPartnerOptinStatusFromResult(t *testing.T) {
	tests := []struct {
		name       string
		result     string
		wantStatus string
		wantMapped bool
	}{
		{"already active", service.SubscriptionResultOptinAlreadyActive, partnerSubscriptionStatusActive, true},
		{"active wait charging", service.SubscriptionResultOptinActiveWaitCharging, partnerSubscriptionStatusActive, true},
		{"preactive wait conf", service.SubscriptionResultOptinPreactiveWaitConf, partnerSubscriptionStatusPreactive, true},
		{"config not found passthrough", service.SubscriptionResultOptinConfigNotFound, "", false},
		{"unknown result", "SOMETHING_ELSE", "", false},
		{"empty result", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, mapped := partnerOptinStatusFromResult(tt.result)
			if status != tt.wantStatus || mapped != tt.wantMapped {
				t.Errorf("partnerOptinStatusFromResult(%q) = (%q, %v), want (%q, %v)",
					tt.result, status, mapped, tt.wantStatus, tt.wantMapped)
			}
		})
	}
}

func TestSubscriptionResultFromResponse(t *testing.T) {
	if got := subscriptionResultFromResponse(nil); got != "" {
		t.Errorf("nil response: got %q, want empty", got)
	}
	if got := subscriptionResultFromResponse(&domain.MTResponse{}); got != "" {
		t.Errorf("nil ResponseData: got %q, want empty", got)
	}
	resp := &domain.MTResponse{ResponseData: map[string]interface{}{"subscriptionResult": "OPTIN_ALREADY_ACTIVE"}}
	if got := subscriptionResultFromResponse(resp); got != "OPTIN_ALREADY_ACTIVE" {
		t.Errorf("got %q, want OPTIN_ALREADY_ACTIVE", got)
	}
	respWrongType := &domain.MTResponse{ResponseData: map[string]interface{}{"subscriptionResult": 42}}
	if got := subscriptionResultFromResponse(respWrongType); got != "" {
		t.Errorf("non-string value: got %q, want empty", got)
	}
}

func TestTransactionIDFromResponse(t *testing.T) {
	if _, ok := transactionIDFromResponse(nil); ok {
		t.Errorf("nil response: expected ok=false")
	}
	if _, ok := transactionIDFromResponse(&domain.MTResponse{}); ok {
		t.Errorf("nil ResponseData: expected ok=false")
	}
	if _, ok := transactionIDFromResponse(&domain.MTResponse{ResponseData: map[string]interface{}{"transactionId": "   "}}); ok {
		t.Errorf("blank transactionId: expected ok=false")
	}
	if _, ok := transactionIDFromResponse(&domain.MTResponse{ResponseData: map[string]interface{}{"transactionId": 123}}); ok {
		t.Errorf("non-string transactionId: expected ok=false")
	}
	id, ok := transactionIDFromResponse(&domain.MTResponse{ResponseData: map[string]interface{}{"transactionId": "txn-1"}})
	if !ok || id != "txn-1" {
		t.Errorf("got (%q, %v), want (txn-1, true)", id, ok)
	}
}

// --- persistPartnerOptinSubscription ---

func TestPersistPartnerOptinSubscription_AlreadyActive_CreatesActive(t *testing.T) {
	repo := newStubTenantPersistRepo()
	h := newPersistenceTestHandler(repo, "42", nil)
	notifier := &stubExistingOptinNotifier{}
	h.WithOptinNotifier(notifier)
	req := domain.MTRequest{UserIdentifier: "233572503330", ProductID: 32535}
	resp := mtResponseWith(service.SubscriptionResultOptinAlreadyActive, "txn-abc")
	resp.ResponseData["externalTxId"] = "external-tx-abc"

	h.persistPartnerOptinSubscription(testRoute(), req, resp)

	if len(repo.createCalls) != 1 {
		t.Fatalf("expected 1 CreateSubscription call, got %d", len(repo.createCalls))
	}
	created := repo.createCalls[0]
	if created.TransactionId != "txn-abc" || created.PartnerRoleId != 42 || created.UserIdentifier != "233572503330" || created.ProductId != 32535 {
		t.Errorf("unexpected CreateSubscription request: %+v", created)
	}
	if len(repo.updateCalls) != 0 {
		t.Errorf("already-active result must not trigger a follow-up status update, got %d calls", len(repo.updateCalls))
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("already-active result must notify once, got %d calls", len(notifier.calls))
	}
	call := notifier.calls[0]
	if call.partnerRole != 42 || call.route.TenantKey != "careerify" {
		t.Errorf("unexpected notification route: %+v", call)
	}
	if call.notification.ExternalTxID != "external-tx-abc" || call.notification.TransactionUUID != "txn-abc" || call.notification.MSISDN != req.UserIdentifier || call.notification.ProductID != req.ProductID {
		t.Errorf("unexpected notification payload: %+v", call.notification)
	}
	if call.notification.MessageType != service.SubscriptionResultOptinAlreadyActive {
		t.Errorf("unexpected MessageType %q, want %q", call.notification.MessageType, service.SubscriptionResultOptinAlreadyActive)
	}
}

// TIMWE reports an existing active subscription as OPTIN_ACTIVE_WAIT_CHARGING
// when its charging cycle is pending, and sends no USER_OPTIN callback for it
// (observed in production 2026-08-19), so this result must notify too.
func TestPersistPartnerOptinSubscription_WaitCharging_Notifies(t *testing.T) {
	repo := newStubTenantPersistRepo()
	h := newPersistenceTestHandler(repo, "42", nil)
	notifier := &stubExistingOptinNotifier{}
	h.WithOptinNotifier(notifier)
	req := domain.MTRequest{UserIdentifier: "233572503330", ProductID: 32535}
	resp := mtResponseWith(service.SubscriptionResultOptinActiveWaitCharging, "txn-abc")
	resp.ResponseData["externalTxId"] = "external-tx-abc"

	h.persistPartnerOptinSubscription(testRoute(), req, resp)

	if len(notifier.calls) != 1 {
		t.Fatalf("wait-charging result must notify once, got %d calls", len(notifier.calls))
	}
	call := notifier.calls[0]
	if call.notification.ExternalTxID != "external-tx-abc" || call.notification.MSISDN != req.UserIdentifier {
		t.Errorf("unexpected notification payload: %+v", call.notification)
	}
	if call.notification.MessageType != service.SubscriptionResultOptinActiveWaitCharging {
		t.Errorf("unexpected MessageType %q, want %q", call.notification.MessageType, service.SubscriptionResultOptinActiveWaitCharging)
	}
}

func TestPersistPartnerOptinSubscription_Preactive_CreatesThenUpdatesStatus(t *testing.T) {
	repo := newStubTenantPersistRepo()
	h := newPersistenceTestHandler(repo, "42", nil)
	notifier := &stubExistingOptinNotifier{}
	h.WithOptinNotifier(notifier)
	req := domain.MTRequest{UserIdentifier: "233572503330", ProductID: 32535}
	resp := mtResponseWith(service.SubscriptionResultOptinPreactiveWaitConf, "txn-abc")
	resp.ResponseData["externalTxId"] = "external-tx-abc"

	h.persistPartnerOptinSubscription(testRoute(), req, resp)

	if len(notifier.calls) != 0 {
		t.Fatalf("preactive (awaiting confirmation) result must not notify, got %d calls", len(notifier.calls))
	}
	if len(repo.createCalls) != 1 {
		t.Fatalf("expected 1 CreateSubscription call, got %d", len(repo.createCalls))
	}
	if len(repo.updateCalls) != 1 {
		t.Fatalf("expected 1 follow-up UpdateSubscriptionStatus call for preactive, got %d", len(repo.updateCalls))
	}
	update := repo.updateCalls[0]
	if update.status != partnerSubscriptionStatusPreactive || update.msisdn != "233572503330" || update.productID != "32535" {
		t.Errorf("unexpected UpdateSubscriptionStatus call: %+v", update)
	}
}

func TestPersistPartnerOptinSubscription_UnmappedResult_SkipsPersistence(t *testing.T) {
	repo := newStubTenantPersistRepo()
	h := newPersistenceTestHandler(repo, "42", nil)
	req := domain.MTRequest{UserIdentifier: "233572503330", ProductID: 32535}
	resp := mtResponseWith(service.SubscriptionResultOptinConfigNotFound, "txn-abc")

	h.persistPartnerOptinSubscription(testRoute(), req, resp)

	if len(repo.createCalls) != 0 || len(repo.updateCalls) != 0 {
		t.Errorf("unmapped subscriptionResult must skip persistence entirely, got creates=%d updates=%d",
			len(repo.createCalls), len(repo.updateCalls))
	}
}

func TestPersistPartnerOptinSubscription_MissingTransactionID_SkipsPersistence(t *testing.T) {
	repo := newStubTenantPersistRepo()
	h := newPersistenceTestHandler(repo, "42", nil)
	req := domain.MTRequest{UserIdentifier: "233572503330", ProductID: 32535}
	resp := mtResponseWith(service.SubscriptionResultOptinAlreadyActive, "")

	h.persistPartnerOptinSubscription(testRoute(), req, resp)

	if len(repo.createCalls) != 0 {
		t.Errorf("missing transactionId must skip persistence, got %d create calls", len(repo.createCalls))
	}
}

func TestPersistPartnerOptinSubscription_RepoNotPersister_SkipsSilently(t *testing.T) {
	// A repo implementing only gatewayTenantLookup (the pre-existing test
	// stub pattern) must not panic and must not be asserted against, since
	// it does not implement subscriptionPersister.
	h := newPersistenceTestHandler(stubRepo(), "42", nil)
	req := domain.MTRequest{UserIdentifier: "233572503330", ProductID: 32535}
	resp := mtResponseWith(service.SubscriptionResultOptinAlreadyActive, "txn-abc")

	h.persistPartnerOptinSubscription(testRoute(), req, resp)
	// No assertion possible beyond "did not panic"; the repo has no
	// createCalls/updateCalls to inspect. Reaching this line is the pass.
}

func TestPersistPartnerOptinSubscription_PartnerRoleResolutionFails_SkipsSilently(t *testing.T) {
	repo := newStubTenantPersistRepo()
	h := newPersistenceTestHandler(repo, "42", errors.New("resolve failed"))
	req := domain.MTRequest{UserIdentifier: "233572503330", ProductID: 32535}
	resp := mtResponseWith(service.SubscriptionResultOptinAlreadyActive, "txn-abc")

	h.persistPartnerOptinSubscription(testRoute(), req, resp)

	if len(repo.createCalls) != 0 {
		t.Errorf("partner role resolution failure must skip persistence, got %d create calls", len(repo.createCalls))
	}
}

func TestPersistPartnerOptinSubscription_CreateSubscriptionError_DoesNotPanic(t *testing.T) {
	repo := newStubTenantPersistRepo()
	repo.createErr = errors.New("db write failed")
	h := newPersistenceTestHandler(repo, "42", nil)
	req := domain.MTRequest{UserIdentifier: "233572503330", ProductID: 32535}
	resp := mtResponseWith(service.SubscriptionResultOptinAlreadyActive, "txn-abc")

	// Must log and swallow the error; the caller (PartnerSubscriptionOptin)
	// must still return the unmodified TIMWE response.
	h.persistPartnerOptinSubscription(testRoute(), req, resp)

	if len(repo.updateCalls) != 0 {
		t.Errorf("a CreateSubscription failure must not trigger the follow-up status update, got %d calls", len(repo.updateCalls))
	}
}

// --- persistPartnerConfirmSubscription ---

func TestPersistPartnerConfirmSubscription_ActivatesSubscription(t *testing.T) {
	repo := newStubTenantPersistRepo()
	h := newPersistenceTestHandler(repo, "42", nil)
	req := domain.SubscriptionConfirmationRequest{UserIdentifier: "233572503330", ProductId: 32535}

	h.persistPartnerConfirmSubscription(testRoute(), req)

	if len(repo.updateCalls) != 1 {
		t.Fatalf("expected 1 UpdateSubscriptionStatus call, got %d", len(repo.updateCalls))
	}
	update := repo.updateCalls[0]
	if update.status != partnerSubscriptionStatusActive || update.msisdn != "233572503330" || update.productID != "32535" {
		t.Errorf("unexpected UpdateSubscriptionStatus call: %+v", update)
	}
}

func TestPersistPartnerConfirmSubscription_RepoNotPersister_SkipsSilently(t *testing.T) {
	h := newPersistenceTestHandler(stubRepo(), "42", nil)
	req := domain.SubscriptionConfirmationRequest{UserIdentifier: "233572503330", ProductId: 32535}
	h.persistPartnerConfirmSubscription(testRoute(), req)
}

func TestPersistPartnerConfirmSubscription_UpdateError_DoesNotPanic(t *testing.T) {
	repo := newStubTenantPersistRepo()
	repo.updateErr = errors.New("db write failed")
	h := newPersistenceTestHandler(repo, "42", nil)
	req := domain.SubscriptionConfirmationRequest{UserIdentifier: "233572503330", ProductId: 32535}
	h.persistPartnerConfirmSubscription(testRoute(), req)
}

// --- persistPartnerOptoutSubscription ---

func TestPersistPartnerOptoutSubscription_DeactivatesSubscription(t *testing.T) {
	repo := newStubTenantPersistRepo()
	h := newPersistenceTestHandler(repo, "42", nil)
	req := domain.UnsubscriptionRequest{UserIdentifier: "233572503330", ProductId: 32535}

	h.persistPartnerOptoutSubscription(testRoute(), req)

	if len(repo.updateCalls) != 1 {
		t.Fatalf("expected 1 UpdateSubscriptionStatus call, got %d", len(repo.updateCalls))
	}
	update := repo.updateCalls[0]
	if update.status != partnerSubscriptionStatusInactive || update.msisdn != "233572503330" || update.productID != "32535" {
		t.Errorf("unexpected UpdateSubscriptionStatus call: %+v", update)
	}
}

func TestPersistPartnerOptoutSubscription_RepoNotPersister_SkipsSilently(t *testing.T) {
	h := newPersistenceTestHandler(stubRepo(), "42", nil)
	req := domain.UnsubscriptionRequest{UserIdentifier: "233572503330", ProductId: 32535}
	h.persistPartnerOptoutSubscription(testRoute(), req)
}

func TestPersistPartnerOptoutSubscription_UpdateError_DoesNotPanic(t *testing.T) {
	repo := newStubTenantPersistRepo()
	repo.updateErr = errors.New("db write failed")
	h := newPersistenceTestHandler(repo, "42", nil)
	req := domain.UnsubscriptionRequest{UserIdentifier: "233572503330", ProductId: 32535}
	h.persistPartnerOptoutSubscription(testRoute(), req)
}
