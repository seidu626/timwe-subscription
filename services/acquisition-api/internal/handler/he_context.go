package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// HESource represents the source of Header Enrichment identity
type HESource string

const (
	HESourceReal      HESource = "REAL"
	HESourceSimulated HESource = "SIMULATED"
	HESourceNone      HESource = "NONE"
)

// HEIdentity represents the resolved HE identity from headers
type HEIdentity struct {
	MSISDN     string
	OperatorID string
	MCC        string
	MNC        string
	Source     HESource
}

// HEContextConfig holds configuration for HE detection
type HEContextConfig struct {
	SimulationEnabled bool
	MSISDNHeaders     []string
	MCCHeader         string
	MNCHeader         string
	OperatorHeader    string
}

// DefaultHEContextConfig returns the default HE context configuration
func DefaultHEContextConfig() *HEContextConfig {
	return &HEContextConfig{
		SimulationEnabled: false,
		MSISDNHeaders: []string{
			"X-MSISDN",
			"X-UP-CALLING-LINE-ID",
			"X_WAP_NETWORK_CLIENT_MSISDN",
		},
		MCCHeader:      "X-MCC",
		MNCHeader:      "X-MNC",
		OperatorHeader: "X-Operator-ID",
	}
}

// HEContextKey is the context key for HE identity
type HEContextKey struct{}

// Request context key for storing HE identity
const heContextKeyName = "he_identity"

// Headers passed from frontend (simulation flow)
const (
	HeaderHESource   = "X-He-Source"
	HeaderHEMSISDN   = "X-He-Msisdn"
	HeaderHEOperator = "X-He-Operator"
	HeaderHEMCC      = "X-He-Mcc"
	HeaderHEMNC      = "X-He-Mnc"
)

// HEContextMiddleware creates middleware for extracting HE identity
type HEContextMiddleware struct {
	config *HEContextConfig
	logger *zap.Logger
}

// NewHEContextMiddleware creates a new HE context middleware
func NewHEContextMiddleware(config *HEContextConfig, logger *zap.Logger) *HEContextMiddleware {
	if config == nil {
		config = DefaultHEContextConfig()
	}
	return &HEContextMiddleware{
		config: config,
		logger: logger,
	}
}

// ExtractIdentity extracts HE identity from the request
// Priority: Real HE headers > Simulated (from frontend) > None
func (m *HEContextMiddleware) ExtractIdentity(ctx *fasthttp.RequestCtx) *HEIdentity {
	// 1. Try real HE headers first (from MNO proxy)
	identity := m.extractRealHEIdentity(ctx)
	if identity != nil {
		// Enrich with operator info from MSISDN prefix if not in headers
		EnrichIdentityWithOperator(identity)
		m.logIdentity("Real HE identity detected", identity)
		return identity
	}

	// 2. Try simulated identity (passed from frontend via headers)
	if m.config.SimulationEnabled {
		identity = m.extractSimulatedIdentity(ctx)
		if identity != nil {
			// Enrich with operator info from MSISDN prefix if not provided
			EnrichIdentityWithOperator(identity)
			m.logIdentity("Simulated HE identity detected", identity)
			return identity
		}
	}

	return nil
}

// extractRealHEIdentity extracts identity from real MNO HE headers
func (m *HEContextMiddleware) extractRealHEIdentity(ctx *fasthttp.RequestCtx) *HEIdentity {
	var msisdn string

	// Check candidate MSISDN headers in order of preference
	for _, headerName := range m.config.MSISDNHeaders {
		value := string(ctx.Request.Header.Peek(headerName))
		if value != "" {
			normalized := normalizeMSISDN(value)
			if isValidMSISDN(normalized) {
				msisdn = normalized
				break
			}
		}
	}

	if msisdn == "" {
		return nil
	}

	return &HEIdentity{
		MSISDN:     msisdn,
		OperatorID: string(ctx.Request.Header.Peek(m.config.OperatorHeader)),
		MCC:        string(ctx.Request.Header.Peek(m.config.MCCHeader)),
		MNC:        string(ctx.Request.Header.Peek(m.config.MNCHeader)),
		Source:     HESourceReal,
	}
}

// extractSimulatedIdentity extracts identity from simulation headers (set by frontend)
func (m *HEContextMiddleware) extractSimulatedIdentity(ctx *fasthttp.RequestCtx) *HEIdentity {
	source := string(ctx.Request.Header.Peek(HeaderHESource))
	msisdn := string(ctx.Request.Header.Peek(HeaderHEMSISDN))

	// Only accept if source indicates simulation and MSISDN is valid
	if source != string(HESourceSimulated) || msisdn == "" {
		return nil
	}

	normalized := normalizeMSISDN(msisdn)
	if !isValidMSISDN(normalized) {
		return nil
	}

	return &HEIdentity{
		MSISDN:     normalized,
		OperatorID: string(ctx.Request.Header.Peek(HeaderHEOperator)),
		MCC:        string(ctx.Request.Header.Peek(HeaderHEMCC)),
		MNC:        string(ctx.Request.Header.Peek(HeaderHEMNC)),
		Source:     HESourceSimulated,
	}
}

// logIdentity logs the detected identity (with MSISDN hashed for privacy)
func (m *HEContextMiddleware) logIdentity(msg string, identity *HEIdentity) {
	m.logger.Info(msg,
		zap.String("he_source", string(identity.Source)),
		zap.String("msisdn_hash", hashMSISDN(identity.MSISDN)),
		zap.String("operator_id", identity.OperatorID),
		zap.String("mcc", identity.MCC),
		zap.String("mnc", identity.MNC),
	)
}

// normalizeMSISDN removes whitespace and leading '+' from MSISDN
func normalizeMSISDN(msisdn string) string {
	// Remove all whitespace
	msisdn = strings.ReplaceAll(msisdn, " ", "")
	msisdn = strings.ReplaceAll(msisdn, "\t", "")
	// Remove leading '+'
	msisdn = strings.TrimPrefix(msisdn, "+")
	return msisdn
}

// isValidMSISDN validates MSISDN format (9-15 digits)
func isValidMSISDN(msisdn string) bool {
	matched, _ := regexp.MatchString(`^\d{9,15}$`, msisdn)
	return matched
}

// hashMSISDN returns SHA256 hash of MSISDN for logging
func hashMSISDN(msisdn string) string {
	hash := sha256.Sum256([]byte(msisdn))
	return hex.EncodeToString(hash[:8]) // First 8 bytes for brevity
}

// GhanaOperator represents a Ghana MNO with prefix mappings
// Based on docs/ghana-header-enrichment-parameters.md
type GhanaOperator struct {
	Name     string
	MCC      string
	MNC      string
	Prefixes []string
}

// GhanaOperators contains the Ghana MNO configurations
// MCC 620 for all Ghana operators
var GhanaOperators = []GhanaOperator{
	{
		Name:     "MTN Ghana",
		MCC:      "620",
		MNC:      "01",
		Prefixes: []string{"23324", "23354", "23355", "23353"},
	},
	{
		Name:     "Telecel Ghana",
		MCC:      "620",
		MNC:      "02",
		Prefixes: []string{"23320", "23350"},
	},
	{
		Name:     "AT Ghana",
		MCC:      "620",
		MNC:      "03",
		Prefixes: []string{"23326", "23327", "23356", "23357"},
	},
}

// DetectGhanaOperator detects the operator from MSISDN prefix
// Returns nil if no matching operator found
func DetectGhanaOperator(msisdn string) *GhanaOperator {
	normalized := normalizeMSISDN(msisdn)
	for i := range GhanaOperators {
		for _, prefix := range GhanaOperators[i].Prefixes {
			if strings.HasPrefix(normalized, prefix) {
				return &GhanaOperators[i]
			}
		}
	}
	return nil
}

// EnrichIdentityWithOperator adds operator info to identity based on MSISDN prefix
// if operator info is not already present from headers
func EnrichIdentityWithOperator(identity *HEIdentity) {
	if identity == nil {
		return
	}

	// Only enrich if operator not already set from headers
	if identity.OperatorID == "" || identity.MCC == "" || identity.MNC == "" {
		if op := DetectGhanaOperator(identity.MSISDN); op != nil {
			if identity.OperatorID == "" {
				identity.OperatorID = op.Name
			}
			if identity.MCC == "" {
				identity.MCC = op.MCC
			}
			if identity.MNC == "" {
				identity.MNC = op.MNC
			}
		}
	}
}

// StoreIdentity stores HE identity in the request context
func StoreIdentity(ctx *fasthttp.RequestCtx, identity *HEIdentity) {
	ctx.SetUserValue(heContextKeyName, identity)
}

// GetIdentity retrieves HE identity from the request context
func GetIdentity(ctx *fasthttp.RequestCtx) *HEIdentity {
	if val := ctx.UserValue(heContextKeyName); val != nil {
		if identity, ok := val.(*HEIdentity); ok {
			return identity
		}
	}
	return nil
}
