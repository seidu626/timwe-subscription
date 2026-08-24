package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/appauth"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

type fakeAppRepo struct {
	listFeedFn     func(ctx context.Context, msisdn string, limit int) ([]domain.AppFeedItem, error)
	getFeedItemFn  func(ctx context.Context, msisdn string, id int64) (*domain.AppFeedItem, error)
	markReadFn     func(ctx context.Context, msisdn string, id int64) error
	upsertDeviceFn func(ctx context.Context, msisdn, tenantKey, fcmToken, platform string) error
	upsertPrefFn   func(ctx context.Context, msisdn, productSlug, channel string) error
	listPrefsFn    func(ctx context.Context, msisdn string) ([]domain.AppNotificationPref, error)
}

func (f *fakeAppRepo) ListFeed(ctx context.Context, msisdn string, limit int) ([]domain.AppFeedItem, error) {
	return f.listFeedFn(ctx, msisdn, limit)
}
func (f *fakeAppRepo) GetFeedItem(ctx context.Context, msisdn string, id int64) (*domain.AppFeedItem, error) {
	return f.getFeedItemFn(ctx, msisdn, id)
}
func (f *fakeAppRepo) MarkRead(ctx context.Context, msisdn string, id int64) error {
	return f.markReadFn(ctx, msisdn, id)
}
func (f *fakeAppRepo) UpsertDevice(ctx context.Context, msisdn, tenantKey, fcmToken, platform string) error {
	return f.upsertDeviceFn(ctx, msisdn, tenantKey, fcmToken, platform)
}
func (f *fakeAppRepo) UpsertNotificationPref(ctx context.Context, msisdn, productSlug, channel string) error {
	return f.upsertPrefFn(ctx, msisdn, productSlug, channel)
}
func (f *fakeAppRepo) ListNotificationPrefs(ctx context.Context, msisdn string) ([]domain.AppNotificationPref, error) {
	return f.listPrefsFn(ctx, msisdn)
}

type fakeValidator struct {
	claims *appauth.Claims
	err    error
}

func (f *fakeValidator) ValidateBearer(authorizationHeader string) (*appauth.Claims, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.claims, nil
}

func newTestCtx(method, body string, authHeader string) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(method)
	if authHeader != "" {
		ctx.Request.Header.Set("Authorization", authHeader)
	}
	if body != "" {
		ctx.Request.SetBody([]byte(body))
	}
	return ctx
}

func errorBody(t *testing.T, ctx *fasthttp.RequestCtx) map[string]map[string]string {
	t.Helper()
	var parsed map[string]map[string]string
	if err := json.Unmarshal(ctx.Response.Body(), &parsed); err != nil {
		t.Fatalf("unmarshal error body: %v (body=%s)", err, ctx.Response.Body())
	}
	return parsed
}

func TestGetFeed_UnauthorizedWithoutValidator(t *testing.T) {
	h := NewAppHandler(&fakeAppRepo{}, nil, zap.NewNop())
	ctx := newTestCtx(fasthttp.MethodGet, "", "")
	h.GetFeed(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", ctx.Response.StatusCode())
	}
	body := errorBody(t, ctx)
	if body["error"]["code"] != "UNAUTHORIZED" {
		t.Errorf("code = %q, want UNAUTHORIZED", body["error"]["code"])
	}
}

func TestGetFeed_UnauthorizedOnInvalidToken(t *testing.T) {
	h := NewAppHandler(&fakeAppRepo{}, &fakeValidator{err: appauth.ErrInvalidToken}, zap.NewNop())
	ctx := newTestCtx(fasthttp.MethodGet, "", "Bearer bad")
	h.GetFeed(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", ctx.Response.StatusCode())
	}
}

func TestGetFeed_ReturnsItemsForAuthenticatedSubject(t *testing.T) {
	claims := &appauth.Claims{}
	claims.Subject = "233241234567"
	claims.Tenant = "careerify"

	published := time.Now()
	linkURL := "https://careerify.example/app"
	ctaLabel := "Open"
	repo := &fakeAppRepo{
		listFeedFn: func(ctx context.Context, msisdn string, limit int) ([]domain.AppFeedItem, error) {
			if msisdn != "233241234567" {
				t.Errorf("msisdn = %q, want 233241234567", msisdn)
			}
			if limit != feedListLimit {
				t.Errorf("limit = %d, want %d", limit, feedListLimit)
			}
			return []domain.AppFeedItem{{ID: 1, Title: "Hi", Body: "Hi", ContentKind: "LINK", LinkURL: &linkURL, CTALabel: &ctaLabel, PublishedAt: published}}, nil
		},
	}
	h := NewAppHandler(repo, &fakeValidator{claims: claims}, zap.NewNop())
	ctx := newTestCtx(fasthttp.MethodGet, "", "Bearer token")
	h.GetFeed(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	var parsed domain.AppFeedListResponse
	if err := json.Unmarshal(ctx.Response.Body(), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.Items) != 1 || parsed.Items[0].ID != 1 {
		t.Errorf("unexpected items: %+v", parsed.Items)
	}
	if parsed.Items[0].ContentKind != "LINK" || parsed.Items[0].LinkURL == nil || *parsed.Items[0].LinkURL != linkURL || parsed.Items[0].CTALabel == nil || *parsed.Items[0].CTALabel != ctaLabel {
		t.Errorf("missing link fields: %+v", parsed.Items[0])
	}
}

func TestGetFeedItem_NotFound(t *testing.T) {
	claims := &appauth.Claims{}
	claims.Subject = "233241234567"
	repo := &fakeAppRepo{
		getFeedItemFn: func(ctx context.Context, msisdn string, id int64) (*domain.AppFeedItem, error) {
			return nil, sql.ErrNoRows
		},
	}
	h := NewAppHandler(repo, &fakeValidator{claims: claims}, zap.NewNop())
	ctx := newTestCtx(fasthttp.MethodGet, "", "Bearer token")
	h.GetFeedItem(ctx, "42")

	if ctx.Response.StatusCode() != fasthttp.StatusNotFound {
		t.Fatalf("status = %d, want 404", ctx.Response.StatusCode())
	}
	if errorBody(t, ctx)["error"]["code"] != "NOT_FOUND" {
		t.Errorf("unexpected error code: %s", ctx.Response.Body())
	}
}

func TestGetFeedItem_InvalidID(t *testing.T) {
	claims := &appauth.Claims{}
	claims.Subject = "233241234567"
	h := NewAppHandler(&fakeAppRepo{}, &fakeValidator{claims: claims}, zap.NewNop())
	ctx := newTestCtx(fasthttp.MethodGet, "", "Bearer token")
	h.GetFeedItem(ctx, "not-a-number")

	if ctx.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Fatalf("status = %d, want 400", ctx.Response.StatusCode())
	}
	if errorBody(t, ctx)["error"]["code"] != "VALIDATION" {
		t.Errorf("unexpected error code: %s", ctx.Response.Body())
	}
}

func TestMarkFeedItemRead_NoContentOnSuccess(t *testing.T) {
	claims := &appauth.Claims{}
	claims.Subject = "233241234567"
	var markedID int64 = -1
	repo := &fakeAppRepo{
		markReadFn: func(ctx context.Context, msisdn string, id int64) error {
			markedID = id
			return nil
		},
	}
	h := NewAppHandler(repo, &fakeValidator{claims: claims}, zap.NewNop())
	ctx := newTestCtx(fasthttp.MethodPost, "", "Bearer token")
	h.MarkFeedItemRead(ctx, "7")

	if ctx.Response.StatusCode() != fasthttp.StatusNoContent {
		t.Fatalf("status = %d, want 204", ctx.Response.StatusCode())
	}
	if markedID != 7 {
		t.Errorf("marked id = %d, want 7", markedID)
	}
}

func TestRegisterDevice_ValidationErrorOnBadPlatform(t *testing.T) {
	claims := &appauth.Claims{}
	claims.Subject = "233241234567"
	h := NewAppHandler(&fakeAppRepo{}, &fakeValidator{claims: claims}, zap.NewNop())
	ctx := newTestCtx(fasthttp.MethodPost, `{"fcm_token":"abc","platform":"windows"}`, "Bearer token")
	h.RegisterDevice(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Fatalf("status = %d, want 400", ctx.Response.StatusCode())
	}
}

func TestRegisterDevice_UpsertsAndReturns204(t *testing.T) {
	claims := &appauth.Claims{}
	claims.Subject = "233241234567"
	claims.Tenant = "careerify"
	var gotArgs [4]string
	repo := &fakeAppRepo{
		upsertDeviceFn: func(ctx context.Context, msisdn, tenantKey, fcmToken, platform string) error {
			gotArgs = [4]string{msisdn, tenantKey, fcmToken, platform}
			return nil
		},
	}
	h := NewAppHandler(repo, &fakeValidator{claims: claims}, zap.NewNop())
	ctx := newTestCtx(fasthttp.MethodPost, `{"fcm_token":"abc123","platform":"ANDROID"}`, "Bearer token")
	h.RegisterDevice(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body=%s)", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	want := [4]string{"233241234567", "careerify", "abc123", "android"}
	if gotArgs != want {
		t.Errorf("upsert args = %v, want %v", gotArgs, want)
	}
}

func TestGetNotificationPrefs_UnauthorizedWithoutValidator(t *testing.T) {
	h := NewAppHandler(&fakeAppRepo{}, nil, zap.NewNop())
	ctx := newTestCtx(fasthttp.MethodGet, "", "")
	h.GetNotificationPrefs(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", ctx.Response.StatusCode())
	}
	if errorBody(t, ctx)["error"]["code"] != "UNAUTHORIZED" {
		t.Errorf("unexpected error code: %s", ctx.Response.Body())
	}
}

func TestGetNotificationPrefs_ReturnsStoredPrefs(t *testing.T) {
	claims := &appauth.Claims{}
	claims.Subject = "233241234567"
	repo := &fakeAppRepo{
		listPrefsFn: func(ctx context.Context, msisdn string) ([]domain.AppNotificationPref, error) {
			if msisdn != "233241234567" {
				t.Errorf("msisdn = %q, want 233241234567", msisdn)
			}
			return []domain.AppNotificationPref{{ProductSlug: "careerify-tips", Channel: "PUSH"}}, nil
		},
	}
	h := NewAppHandler(repo, &fakeValidator{claims: claims}, zap.NewNop())
	ctx := newTestCtx(fasthttp.MethodGet, "", "Bearer token")
	h.GetNotificationPrefs(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	var parsed domain.AppNotificationPrefsResponse
	if err := json.Unmarshal(ctx.Response.Body(), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.Prefs) != 1 || parsed.Prefs[0].ProductSlug != "careerify-tips" || parsed.Prefs[0].Channel != "PUSH" {
		t.Errorf("unexpected prefs: %+v", parsed.Prefs)
	}
}

func TestGetNotificationPrefs_EmptyIsPrefsArrayNotNull(t *testing.T) {
	claims := &appauth.Claims{}
	claims.Subject = "233241234567"
	repo := &fakeAppRepo{
		listPrefsFn: func(ctx context.Context, msisdn string) ([]domain.AppNotificationPref, error) {
			return []domain.AppNotificationPref{}, nil
		},
	}
	h := NewAppHandler(repo, &fakeValidator{claims: claims}, zap.NewNop())
	ctx := newTestCtx(fasthttp.MethodGet, "", "Bearer token")
	h.GetNotificationPrefs(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status = %d, want 200", ctx.Response.StatusCode())
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(ctx.Response.Body(), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(parsed["prefs"]) != "[]" {
		t.Errorf("prefs = %s, want []", parsed["prefs"])
	}
}

func TestGetNotificationPrefs_ProviderErrorOnRepoFailure(t *testing.T) {
	claims := &appauth.Claims{}
	claims.Subject = "233241234567"
	repo := &fakeAppRepo{
		listPrefsFn: func(ctx context.Context, msisdn string) ([]domain.AppNotificationPref, error) {
			return nil, sql.ErrConnDone
		},
	}
	h := NewAppHandler(repo, &fakeValidator{claims: claims}, zap.NewNop())
	ctx := newTestCtx(fasthttp.MethodGet, "", "Bearer token")
	h.GetNotificationPrefs(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", ctx.Response.StatusCode())
	}
	if errorBody(t, ctx)["error"]["code"] != "PROVIDER_ERROR" {
		t.Errorf("unexpected error code: %s", ctx.Response.Body())
	}
}

func TestSetNotificationPrefs_ValidationErrorOnBadChannel(t *testing.T) {
	claims := &appauth.Claims{}
	claims.Subject = "233241234567"
	h := NewAppHandler(&fakeAppRepo{}, &fakeValidator{claims: claims}, zap.NewNop())
	ctx := newTestCtx(fasthttp.MethodPut, `{"product_slug":"careerify-tips","channel":"CARRIER_PIGEON"}`, "Bearer token")
	h.SetNotificationPrefs(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Fatalf("status = %d, want 400", ctx.Response.StatusCode())
	}
}

func TestSetNotificationPrefs_Success(t *testing.T) {
	claims := &appauth.Claims{}
	claims.Subject = "233241234567"
	var gotChannel string
	repo := &fakeAppRepo{
		upsertPrefFn: func(ctx context.Context, msisdn, productSlug, channel string) error {
			gotChannel = channel
			return nil
		},
	}
	h := NewAppHandler(repo, &fakeValidator{claims: claims}, zap.NewNop())
	ctx := newTestCtx(fasthttp.MethodPut, `{"product_slug":"careerify-tips","channel":"push"}`, "Bearer token")
	h.SetNotificationPrefs(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body=%s)", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if gotChannel != "PUSH" {
		t.Errorf("channel = %q, want PUSH", gotChannel)
	}
}
