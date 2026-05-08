package handler

import (
	"net"
	"testing"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func TestExtractIdentity_RealHETrustedProxy(t *testing.T) {
	config := &HEContextConfig{
		SimulationEnabled: false,
		MSISDNHeaders:     []string{"X-MSISDN"},
		MCCHeader:         "X-MCC",
		MNCHeader:         "X-MNC",
		OperatorHeader:    "X-Operator-ID",
		TrustedProxyCIDRs: []string{"10.0.0.0/8"},
	}
	middleware := NewHEContextMiddleware(config, zap.NewNop())

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("X-MSISDN", "233241234567")
	ctx.SetRemoteAddr(&net.TCPAddr{IP: net.ParseIP("10.1.2.3"), Port: 1234})

	identity := middleware.ExtractIdentity(&ctx)
	if identity == nil {
		t.Fatal("expected HE identity for trusted proxy")
	}
	if identity.Source != HESourceReal {
		t.Fatalf("expected source REAL, got %s", identity.Source)
	}
	if identity.OperatorID != "MTN Ghana" {
		t.Fatalf("expected operator MTN Ghana, got %s", identity.OperatorID)
	}
}

func TestExtractIdentity_RealHEUntrustedProxy(t *testing.T) {
	config := &HEContextConfig{
		SimulationEnabled: false,
		MSISDNHeaders:     []string{"X-MSISDN"},
		MCCHeader:         "X-MCC",
		MNCHeader:         "X-MNC",
		OperatorHeader:    "X-Operator-ID",
		TrustedProxyCIDRs: []string{"10.0.0.0/8"},
	}
	middleware := NewHEContextMiddleware(config, zap.NewNop())

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("X-MSISDN", "233241234567")
	ctx.SetRemoteAddr(&net.TCPAddr{IP: net.ParseIP("192.168.1.10"), Port: 1234})

	identity := middleware.ExtractIdentity(&ctx)
	if identity != nil {
		t.Fatal("expected no HE identity for untrusted proxy")
	}
}

func TestExtractIdentity_SimulatedEnabled(t *testing.T) {
	config := &HEContextConfig{
		SimulationEnabled: true,
		MSISDNHeaders:     []string{"X-MSISDN"},
		MCCHeader:         "X-MCC",
		MNCHeader:         "X-MNC",
		OperatorHeader:    "X-Operator-ID",
	}
	middleware := NewHEContextMiddleware(config, zap.NewNop())

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set(HeaderHESource, string(HESourceSimulated))
	ctx.Request.Header.Set(HeaderHEMSISDN, "233201234567")
	ctx.SetRemoteAddr(&net.TCPAddr{IP: net.ParseIP("192.168.1.10"), Port: 1234})

	identity := middleware.ExtractIdentity(&ctx)
	if identity == nil {
		t.Fatal("expected HE identity for simulated headers")
	}
	if identity.Source != HESourceSimulated {
		t.Fatalf("expected source SIMULATED, got %s", identity.Source)
	}
	if identity.OperatorID != "Telecel Ghana" {
		t.Fatalf("expected operator Telecel Ghana, got %s", identity.OperatorID)
	}
}

func TestDetectGhanaOperator_MTN(t *testing.T) {
	testCases := []string{
		"233240123456",
		"233540123456",
		"233550123456",
	}

	for _, msisdn := range testCases {
		operator := DetectGhanaOperator(msisdn)
		if operator == nil {
			t.Fatalf("expected operator for MSISDN %s", msisdn)
		}
		if operator.Name != "MTN Ghana" {
			t.Fatalf("expected MTN Ghana for MSISDN %s, got %s", msisdn, operator.Name)
		}
	}
}

func TestDetectGhanaOperator_Telecel(t *testing.T) {
	testCases := []string{
		"233200123456",
		"233500123456",
	}

	for _, msisdn := range testCases {
		operator := DetectGhanaOperator(msisdn)
		if operator == nil {
			t.Fatalf("expected operator for MSISDN %s", msisdn)
		}
		if operator.Name != "Telecel Ghana" {
			t.Fatalf("expected Telecel Ghana for MSISDN %s, got %s", msisdn, operator.Name)
		}
	}
}

func TestDetectGhanaOperator_AT(t *testing.T) {
	testCases := []string{
		"233260123456",
		"233270123456",
	}

	for _, msisdn := range testCases {
		operator := DetectGhanaOperator(msisdn)
		if operator == nil {
			t.Fatalf("expected operator for MSISDN %s", msisdn)
		}
		if operator.Name != "AT Ghana" {
			t.Fatalf("expected AT Ghana for MSISDN %s, got %s", msisdn, operator.Name)
		}
	}
}

func TestDetectGhanaOperator_Unknown(t *testing.T) {
	operator := DetectGhanaOperator("254701234567")
	if operator != nil {
		t.Fatalf("expected no operator, got %s", operator.Name)
	}
}

func TestNormalizeMSISDN(t *testing.T) {
	normalized := normalizeMSISDN(" +233 24\t012 3456 ")
	if normalized != "233240123456" {
		t.Fatalf("expected normalized MSISDN 233240123456, got %s", normalized)
	}
}

func TestHECampaignRouteFromPathSupportsTenantRoute(t *testing.T) {
	if got := heCampaignRouteFromPath("/v1/he/bootstrap/campaign/tenant-a/daily"); got != "tenant-a/daily" {
		t.Fatalf("expected tenant campaign route, got %q", got)
	}
	if got := heCampaignRouteFromPath("/v1/he/bootstrap/campaign/daily"); got != "daily" {
		t.Fatalf("expected legacy campaign route, got %q", got)
	}
	if got := heCampaignRouteFromPath("/v1/he/bootstrap/campaign/tenant-a/../daily"); got != "" {
		t.Fatalf("expected unsafe route to be rejected, got %q", got)
	}
}

func TestIsValidMSISDN(t *testing.T) {
	validCases := []string{
		"123456789",
		"123456789012345",
	}
	for _, msisdn := range validCases {
		if !isValidMSISDN(msisdn) {
			t.Fatalf("expected MSISDN %s to be valid", msisdn)
		}
	}

	invalidCases := []string{
		"12345678",
		"1234567890123456",
		"12345abcde",
		"123 456789",
	}
	for _, msisdn := range invalidCases {
		if isValidMSISDN(msisdn) {
			t.Fatalf("expected MSISDN %s to be invalid", msisdn)
		}
	}
}

func TestExtractIdentity_RealHeaders(t *testing.T) {
	config := &HEContextConfig{
		SimulationEnabled: false,
		MSISDNHeaders:     []string{"X-MSISDN"},
		MCCHeader:         "X-MCC",
		MNCHeader:         "X-MNC",
		OperatorHeader:    "X-Operator-ID",
		TrustedProxyCIDRs: []string{"10.0.0.0/8"},
	}
	middleware := NewHEContextMiddleware(config, zap.NewNop())

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("X-MSISDN", "233241234567")
	ctx.SetRemoteAddr(&net.TCPAddr{IP: net.ParseIP("10.2.3.4"), Port: 1234})

	identity := middleware.ExtractIdentity(&ctx)
	if identity == nil {
		t.Fatal("expected HE identity for real headers")
	}
	if identity.Source != HESourceReal {
		t.Fatalf("expected source REAL, got %s", identity.Source)
	}
}

func TestExtractIdentity_SimulatedHeaders(t *testing.T) {
	config := &HEContextConfig{
		SimulationEnabled: true,
		MSISDNHeaders:     []string{"X-MSISDN"},
		MCCHeader:         "X-MCC",
		MNCHeader:         "X-MNC",
		OperatorHeader:    "X-Operator-ID",
	}
	middleware := NewHEContextMiddleware(config, zap.NewNop())

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set(HeaderHESource, string(HESourceSimulated))
	ctx.Request.Header.Set(HeaderHEMSISDN, "233201234567")

	identity := middleware.ExtractIdentity(&ctx)
	if identity == nil {
		t.Fatal("expected HE identity for simulated headers")
	}
	if identity.Source != HESourceSimulated {
		t.Fatalf("expected source SIMULATED, got %s", identity.Source)
	}
}

func TestExtractIdentity_NoHeaders(t *testing.T) {
	config := &HEContextConfig{
		SimulationEnabled: true,
		MSISDNHeaders:     []string{"X-MSISDN"},
		MCCHeader:         "X-MCC",
		MNCHeader:         "X-MNC",
		OperatorHeader:    "X-Operator-ID",
	}
	middleware := NewHEContextMiddleware(config, zap.NewNop())

	var ctx fasthttp.RequestCtx
	identity := middleware.ExtractIdentity(&ctx)
	if identity != nil {
		t.Fatal("expected no HE identity when headers are missing")
	}
}

func TestEnrichIdentityWithOperator(t *testing.T) {
	identity := &HEIdentity{
		MSISDN: "233240123456",
		Source: HESourceReal,
	}

	EnrichIdentityWithOperator(identity)

	if identity.OperatorID != "MTN Ghana" {
		t.Fatalf("expected operator MTN Ghana, got %s", identity.OperatorID)
	}
	if identity.MCC != "620" {
		t.Fatalf("expected MCC 620, got %s", identity.MCC)
	}
	if identity.MNC != "01" {
		t.Fatalf("expected MNC 01, got %s", identity.MNC)
	}
}
