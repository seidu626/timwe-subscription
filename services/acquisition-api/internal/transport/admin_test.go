package transport

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/seidu626/subscription-manager/common/auth/auth0jwt"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/valyala/fasthttp"
)

func TestAdminRequireStoresTenantIdentity(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	access := &adminAccess{validator: validator}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss":                    "https://example.auth0.com/",
		"aud":                    []string{"api"},
		"sub":                    "auth0|admin",
		"iat":                    time.Now().Unix(),
		"exp":                    time.Now().Add(time.Hour).Unix(),
		"tenant_id":              "tenant-123",
		"https://platform/roles": []string{"tenant_admin"},
	})

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("Authorization", "Bearer "+token)

	if !access.require(&ctx) {
		t.Fatalf("expected admin auth to pass, status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	value := ctx.UserValue(tenantctx.FastHTTPUserValueKey)
	identity, ok := value.(tenantctx.Identity)
	if !ok {
		t.Fatalf("expected tenant identity in user values, got %#v", value)
	}
	if identity.TenantID != "tenant-123" || !identity.HasRole("tenant_admin") || identity.TrustSource != tenantctx.TrustSourceJWT {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestAdminRequireAppliesBootstrapPlatformEmailAndSelectedTenant(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	access := &adminAccess{
		validator: validator,
		bootstrapPlatformEmails: map[string]struct{}{
			"almauricin@gmail.com": {},
		},
	}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss":            "https://example.auth0.com/",
		"aud":            []string{"api"},
		"sub":            "auth0|bootstrap-admin",
		"email":          "AlMauricin@gmail.com",
		"email_verified": true,
		"iat":            time.Now().Unix(),
		"exp":            time.Now().Add(time.Hour).Unix(),
	})

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	ctx.Request.Header.Set(tenantctx.HeaderTenantKey, "nrg")

	if !access.require(&ctx) {
		t.Fatalf("expected admin auth to pass, status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	identity := ctx.UserValue(tenantctx.FastHTTPUserValueKey).(tenantctx.Identity)
	if !identity.PlatformScoped || !identity.HasPermission("platform:all_tenants") || identity.TenantKey != "nrg" {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestAdminRequireAppliesBootstrapPlatformEmailWhenEmailVerifiedMissing(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	access := &adminAccess{
		validator: validator,
		bootstrapPlatformEmails: map[string]struct{}{
			"almauricin@gmail.com": {},
		},
	}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss":   "https://example.auth0.com/",
		"aud":   []string{"api"},
		"sub":   "auth0|bootstrap-admin",
		"email": "almauricin@gmail.com",
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(time.Hour).Unix(),
	})

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	ctx.Request.Header.Set(tenantctx.HeaderTenantKey, "nrg")

	if !access.require(&ctx) {
		t.Fatalf("expected admin auth to pass, status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	identity := ctx.UserValue(tenantctx.FastHTTPUserValueKey).(tenantctx.Identity)
	if !identity.PlatformScoped || !identity.HasPermission("platform:all_tenants") || identity.TenantKey != "nrg" {
		t.Fatalf("identity = %#v", identity)
	}
	if identity.EmailVerifiedSet {
		t.Fatalf("email_verified should be absent, identity = %#v", identity)
	}
}

func TestAdminRequireAppliesBootstrapPlatformSubjectAndSelectedTenant(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	access := &adminAccess{
		validator: validator,
		bootstrapPlatformSubjects: map[string]struct{}{
			"google-oauth2|118328773120143328716": {},
		},
	}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss": "https://example.auth0.com/",
		"aud": []string{"api"},
		"sub": "google-oauth2|118328773120143328716",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	ctx.Request.Header.Set(tenantctx.HeaderTenantKey, "nrg")

	if !access.require(&ctx) {
		t.Fatalf("expected admin auth to pass, status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	identity := ctx.UserValue(tenantctx.FastHTTPUserValueKey).(tenantctx.Identity)
	if !identity.PlatformScoped || !identity.HasPermission("platform:all_tenants") || identity.TenantKey != "nrg" {
		t.Fatalf("identity = %#v", identity)
	}
	if identity.Email != "" {
		t.Fatalf("test token should not rely on email claim, identity = %#v", identity)
	}
}

func TestAdminRequireIgnoresSelectedTenantHeaderForUnscopedIdentity(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	access := &adminAccess{
		validator: validator,
		bootstrapPlatformEmails: map[string]struct{}{
			"almauricin@gmail.com": {},
		},
	}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss":   "https://example.auth0.com/",
		"aud":   []string{"api"},
		"sub":   "auth0|ordinary-admin",
		"email": "ordinary@example.com",
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(time.Hour).Unix(),
	})

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	ctx.Request.Header.Set(tenantctx.HeaderTenantKey, "tenant-b")

	if !access.require(&ctx) {
		t.Fatalf("expected admin auth to pass, status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	identity := ctx.UserValue(tenantctx.FastHTTPUserValueKey).(tenantctx.Identity)
	if identity.PlatformScoped || identity.TenantKey != "" {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestAdminRequireDoesNotBootstrapUnverifiedEmail(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	access := &adminAccess{
		validator: validator,
		bootstrapPlatformEmails: map[string]struct{}{
			"almauricin@gmail.com": {},
		},
	}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss":            "https://example.auth0.com/",
		"aud":            []string{"api"},
		"sub":            "auth0|unverified-bootstrap-admin",
		"email":          "almauricin@gmail.com",
		"email_verified": false,
		"iat":            time.Now().Unix(),
		"exp":            time.Now().Add(time.Hour).Unix(),
	})

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	ctx.Request.Header.Set(tenantctx.HeaderTenantKey, "nrg")

	if !access.require(&ctx) {
		t.Fatalf("expected admin auth to pass, status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	identity := ctx.UserValue(tenantctx.FastHTTPUserValueKey).(tenantctx.Identity)
	if identity.PlatformScoped || identity.TenantKey != "" {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestAdminRequireStampsSingleMembershipTenant(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	var capturedSubject, capturedEmail string
	access := &adminAccess{
		validator: validator,
		memberLookup: func(subject, email string) ([]MemberTenant, error) {
			capturedSubject, capturedEmail = subject, email
			return []MemberTenant{{ID: "66d39a9a-f1ef-4721-a31c-5bb966d25c3d", TenantKey: "nrg"}}, nil
		},
	}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss":            "https://example.auth0.com/",
		"aud":            []string{"api"},
		"sub":            "google-oauth2|118328773120143328716",
		"email":          "tenant-admin@example.com",
		"email_verified": true,
		"iat":            time.Now().Unix(),
		"exp":            time.Now().Add(time.Hour).Unix(),
	})

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("Authorization", "Bearer "+token)

	if !access.require(&ctx) {
		t.Fatalf("expected admin auth to pass, status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	identity := ctx.UserValue(tenantctx.FastHTTPUserValueKey).(tenantctx.Identity)
	if identity.TenantID != "66d39a9a-f1ef-4721-a31c-5bb966d25c3d" || identity.TenantKey != "nrg" {
		t.Fatalf("identity = %#v", identity)
	}
	if identity.PlatformScoped {
		t.Fatalf("non-platform user must not be promoted, identity = %#v", identity)
	}
	if capturedSubject != "google-oauth2|118328773120143328716" || capturedEmail != "tenant-admin@example.com" {
		t.Fatalf("lookup called with subject=%q email=%q", capturedSubject, capturedEmail)
	}
}

func TestAdminRequireLeavesIdentityEmptyWhenNoMembership(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	access := &adminAccess{
		validator: validator,
		memberLookup: func(subject, email string) ([]MemberTenant, error) {
			return nil, nil
		},
	}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss":            "https://example.auth0.com/",
		"aud":            []string{"api"},
		"sub":            "auth0|stranger",
		"email":          "stranger@example.com",
		"email_verified": true,
		"iat":            time.Now().Unix(),
		"exp":            time.Now().Add(time.Hour).Unix(),
	})

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("Authorization", "Bearer "+token)

	if !access.require(&ctx) {
		t.Fatalf("expected admin auth to pass (handler will 403), status=%d", ctx.Response.StatusCode())
	}
	identity := ctx.UserValue(tenantctx.FastHTTPUserValueKey).(tenantctx.Identity)
	if identity.HasTenant() {
		t.Fatalf("non-member must not get tenant context, identity = %#v", identity)
	}
}

func TestAdminRequireDisambiguatesMultipleMembershipsByHeader(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	access := &adminAccess{
		validator: validator,
		memberLookup: func(subject, email string) ([]MemberTenant, error) {
			return []MemberTenant{
				{ID: "id-a", TenantKey: "tenant-a"},
				{ID: "id-b", TenantKey: "tenant-b"},
			}, nil
		},
	}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss":            "https://example.auth0.com/",
		"aud":            []string{"api"},
		"sub":            "auth0|multi-admin",
		"email":          "multi@example.com",
		"email_verified": true,
		"iat":            time.Now().Unix(),
		"exp":            time.Now().Add(time.Hour).Unix(),
	})

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	ctx.Request.Header.Set(tenantctx.HeaderTenantKey, "Tenant-B")

	if !access.require(&ctx) {
		t.Fatalf("expected admin auth to pass, status=%d", ctx.Response.StatusCode())
	}
	identity := ctx.UserValue(tenantctx.FastHTTPUserValueKey).(tenantctx.Identity)
	if identity.TenantID != "id-b" || identity.TenantKey != "tenant-b" {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestAdminRequireRejectsHeaderNotMatchingMemberships(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	access := &adminAccess{
		validator: validator,
		memberLookup: func(subject, email string) ([]MemberTenant, error) {
			return []MemberTenant{
				{ID: "id-a", TenantKey: "tenant-a"},
				{ID: "id-b", TenantKey: "tenant-b"},
			}, nil
		},
	}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss":            "https://example.auth0.com/",
		"aud":            []string{"api"},
		"sub":            "auth0|multi-admin",
		"email":          "multi@example.com",
		"email_verified": true,
		"iat":            time.Now().Unix(),
		"exp":            time.Now().Add(time.Hour).Unix(),
	})

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	ctx.Request.Header.Set(tenantctx.HeaderTenantKey, "tenant-c")

	if !access.require(&ctx) {
		t.Fatalf("expected admin auth to pass, status=%d", ctx.Response.StatusCode())
	}
	identity := ctx.UserValue(tenantctx.FastHTTPUserValueKey).(tenantctx.Identity)
	if identity.HasTenant() {
		t.Fatalf("must not stamp a tenant the user is not a member of, identity = %#v", identity)
	}
}

func TestAdminRequireSkipsMembershipLookupForPlatformIdentity(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	called := false
	access := &adminAccess{
		validator: validator,
		bootstrapPlatformSubjects: map[string]struct{}{
			"auth0|platform-admin": {},
		},
		memberLookup: func(subject, email string) ([]MemberTenant, error) {
			called = true
			return nil, nil
		},
	}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss": "https://example.auth0.com/",
		"aud": []string{"api"},
		"sub": "auth0|platform-admin",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	ctx.Request.Header.Set(tenantctx.HeaderTenantKey, "nrg")

	if !access.require(&ctx) {
		t.Fatalf("expected admin auth to pass, status=%d", ctx.Response.StatusCode())
	}
	if called {
		t.Fatal("membership lookup must be skipped for platform-scoped identity")
	}
	identity := ctx.UserValue(tenantctx.FastHTTPUserValueKey).(tenantctx.Identity)
	if !identity.PlatformScoped || identity.TenantKey != "nrg" {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestAdminRequireMembershipLookupErrorDoesNotEscalate(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	access := &adminAccess{
		validator: validator,
		memberLookup: func(subject, email string) ([]MemberTenant, error) {
			return nil, errAdminTestLookup
		},
	}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss":            "https://example.auth0.com/",
		"aud":            []string{"api"},
		"sub":            "auth0|admin",
		"email":          "admin@example.com",
		"email_verified": true,
		"iat":            time.Now().Unix(),
		"exp":            time.Now().Add(time.Hour).Unix(),
	})

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("Authorization", "Bearer "+token)

	if !access.require(&ctx) {
		t.Fatalf("expected admin auth to pass (handler will 403), status=%d", ctx.Response.StatusCode())
	}
	identity := ctx.UserValue(tenantctx.FastHTTPUserValueKey).(tenantctx.Identity)
	if identity.HasTenant() {
		t.Fatalf("identity must remain empty when lookup errors, identity = %#v", identity)
	}
}

func TestAdminRequireSkipsEmailInLookupWhenUnverified(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	var capturedEmail string
	access := &adminAccess{
		validator: validator,
		memberLookup: func(subject, email string) ([]MemberTenant, error) {
			capturedEmail = email
			return nil, nil
		},
	}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss":            "https://example.auth0.com/",
		"aud":            []string{"api"},
		"sub":            "auth0|admin",
		"email":          "admin@example.com",
		"email_verified": false,
		"iat":            time.Now().Unix(),
		"exp":            time.Now().Add(time.Hour).Unix(),
	})

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("Authorization", "Bearer "+token)

	if !access.require(&ctx) {
		t.Fatalf("expected admin auth to pass, status=%d", ctx.Response.StatusCode())
	}
	if capturedEmail != "" {
		t.Fatalf("unverified email must not be forwarded to membership lookup, got %q", capturedEmail)
	}
}

var errAdminTestLookup = adminTestError("lookup failed")

type adminTestError string

func (e adminTestError) Error() string { return string(e) }

func TestBootstrapPlatformEmailSetDefaultsClosed(t *testing.T) {
	if got := bootstrapPlatformEmailSet(""); len(got) != 0 {
		t.Fatalf("empty bootstrap config must not grant platform scope, got %#v", got)
	}
}

func TestBootstrapPlatformSubjectSetDefaultsClosed(t *testing.T) {
	if got := bootstrapPlatformSubjectSet(""); len(got) != 0 {
		t.Fatalf("empty bootstrap subject config must not grant platform scope, got %#v", got)
	}
}

func TestAdminRequireRejectsAudienceMismatchBeforeIdentity(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	access := &adminAccess{validator: validator}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss": "https://example.auth0.com/",
		"aud": []string{"wrong"},
		"sub": "auth0|admin",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("Authorization", "Bearer "+token)

	if access.require(&ctx) {
		t.Fatal("expected admin auth to reject audience mismatch")
	}
	if ctx.UserValue(tenantctx.FastHTTPUserValueKey) != nil {
		t.Fatal("tenant identity should not be attached on auth failure")
	}
	if ctx.Response.StatusCode() != fasthttp.StatusUnauthorized {
		t.Fatalf("status = %d", ctx.Response.StatusCode())
	}
}

func TestIsTenantCampaignPathRequiresTenantAndSlug(t *testing.T) {
	if !isTenantCampaignPath("/v1/campaigns/tenant-a/daily") {
		t.Fatal("expected tenant campaign path to match")
	}
	if isTenantCampaignPath("/v1/campaigns/daily") {
		t.Fatal("single-segment campaign path must not match tenant campaign route")
	}
}

func mustAdminRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return key
}

func mustAdminToken(t *testing.T, privateKey *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}
