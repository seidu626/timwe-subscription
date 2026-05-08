package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/seidu626/subscription-manager/subscription-external/internal/utils"

	"sync"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/repository"
	"github.com/sony/gobreaker"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// RenewalServiceInterface defines the interface for renewal operations
type RenewalServiceInterface interface {
	SendRenewalRequest(ctx context.Context, msisdn string, product *domain.Product, channel string) (*domain.RenewalResponse, error)
	OptOutForRenewal(ctx context.Context, msisdn string, product *domain.Product, cycle *domain.RenewalCycle) error
	OptInForRenewal(ctx context.Context, msisdn string, product *domain.Product, entryChannel string, cycle *domain.RenewalCycle) error
	EvaluateChurnPolicy(ctx context.Context, msisdn string, productID string) domain.ChurnAction
	ChurnSubscription(ctx context.Context, msisdn string, productID string, reason string) error
}

// Response codes from TIMWE API
const (
	ResponseCodeSuccess       = "SUCCESS"
	ResponseCodeInternalError = "INTERNAL_ERROR"
	ResponseCodeBlacklisted   = "BLACKLISTED"
)

// Subscription result codes from TIMWE API
const (
	SubscriptionResultOptinAlreadyActive      = "OPTIN_ALREADY_ACTIVE"
	SubscriptionResultOptinActiveWaitCharging = "OPTIN_ACTIVE_WAIT_CHARGING"
	SubscriptionResultOptinPreactiveWaitConf  = "OPTIN_PREACTIVE_WAIT_CONF"
	SubscriptionResultOptinConfigNotFound     = "OPTIN_CONFIG_NOT_FOUND"
	SubscriptionResultInvalidMsisdn           = "INVALID_MSISDN"
	SubscriptionResultInvalidEntryFlowChannel = "INVALID_ENTRY_FLOW_CHANNEL"
	SubscriptionResultOptoutCanceledOK        = "OPTOUT_CANCELED_OK"
	SubscriptionResultOptoutNoSub             = "OPTOUT_NO_SUB"
	SubscriptionResultNull                    = "null"
)

// Subscription error messages
const (
	SubscriptionErrorAlreadyActive           = "Already Active"
	SubscriptionErrorActiveWaitCharging      = "Active and Wait Charging"
	SubscriptionErrorOptinConfigNotFound     = "Optin configuration not found!"
	SubscriptionErrorInvalidMsisdn           = "Invalid MSISDN"
	SubscriptionErrorInvalidEntryFlowChannel = "Invalid Entry Flow Channel"
	SubscriptionErrorOptoutOneSuccess        = "Optout one success"
	SubscriptionErrorOptoutNonExistent       = "Optout non existent subscription"
)

const (
	timweDefaultEntryChannel = "INTERNAL"
	timweDefaultClientIP     = "INTERNAL"
	timweDefaultMSISDNType   = "MSISDN"
)

type timweOptinPayload struct {
	UserIdentifier     string `json:"userIdentifier"`
	UserIdentifierType string `json:"userIdentifierType"`
	ProductID          int    `json:"productId"`
	MCC                string `json:"mcc"`
	MNC                string `json:"mnc"`
	EntryChannel       string `json:"entryChannel"`
	LargeAccount       string `json:"largeAccount"`
	SubKeyword         string `json:"subKeyword"`
	TrackingID         string `json:"trackingId"`
	ClientIP           string `json:"clientIp"`
	CampaignURL        string `json:"campaignUrl"`
}

type timweOptinConfirmPayload struct {
	UserIdentifier      string `json:"userIdentifier"`
	UserIdentifierType  string `json:"userIdentifierType"`
	ProductID           int    `json:"productId"`
	MCC                 string `json:"mcc"`
	MNC                 string `json:"mnc"`
	EntryChannel        string `json:"entryChannel"`
	ClientIP            string `json:"clientIp"`
	TransactionAuthCode string `json:"transactionAuthCode"`
}

type timweOptoutPayload struct {
	UserIdentifier        string `json:"userIdentifier"`
	UserIdentifierType    string `json:"userIdentifierType"`
	ProductID             int    `json:"productId"`
	MCC                   string `json:"mcc"`
	MNC                   string `json:"mnc"`
	EntryChannel          string `json:"entryChannel"`
	LargeAccount          string `json:"largeAccount"`
	SubKeyword            string `json:"subKeyword"`
	TrackingID            string `json:"trackingId"`
	ClientIP              string `json:"clientIp"`
	ControlKeyword        string `json:"controlKeyword"`
	ControlServiceKeyword string `json:"controlServiceKeyword"`
	SubID                 int    `json:"subId"`
	CancelReason          int    `json:"cancelReason"`
	CancelSource          int    `json:"cancelSource"`
}

type timweStatusPayload struct {
	UserIdentifier        string `json:"userIdentifier"`
	UserIdentifierType    string `json:"userIdentifierType"`
	ProductID             int    `json:"productId"`
	MCC                   string `json:"mcc"`
	MNC                   string `json:"mnc"`
	EntryChannel          string `json:"entryChannel"`
	ClientIP              string `json:"clientIp"`
	ControlKeyword        string `json:"controlKeyword"`
	ControlServiceKeyword string `json:"controlServiceKeyword"`
	SubID                 int    `json:"subId"`
}

type SubscriptionService struct {
	repo               repository.SubscriptionRepositoryInterface
	logger             *zap.Logger
	productRepo        *repository.ProductRepository
	UserBaseRepository repository.UserBaseRepositoryInterface
	client             *fasthttp.Client
	networkClient      *utils.NetworkResilientClient // Enhanced network client
	config             *config.Config
	tenantRouter       *TenantProviderRouter
	circuitBreaker     *gobreaker.TwoStepCircuitBreaker
	productsCache      sync.Map                // key: joined productIds or "all" -> []*domain.Product
	bulkhead           chan struct{}           // semaphore for external calls
	renewalService     RenewalServiceInterface // Interface for renewal operations
	msisdnValidator    *utils.MSISDNValidator  // MSISDN validation service
	cleanupTicker      *time.Ticker            // Ticker for periodic cleanup
}

func NewSubscriptionService(logger *zap.Logger, repo repository.SubscriptionRepositoryInterface,
	productRepo *repository.ProductRepository,
	userBaseRepository repository.UserBaseRepositoryInterface,
	cfg *config.Config, renewalService RenewalServiceInterface) *SubscriptionService {

	// Optimize HTTP client for high-volume processing
	maxConnections := cfg.Application.TIMWE.MaxConnections
	if maxConnections < 1000 {
		maxConnections = 1000 // Ensure minimum for high volume
	}

	// Enhanced HTTP client with proper timeouts and connection management
	client := &fasthttp.Client{
		MaxConnsPerHost:          maxConnections,
		MaxIdleConnDuration:      30 * time.Second,
		ReadTimeout:              cfg.Application.TIMWE.Timeout,
		WriteTimeout:             cfg.Application.TIMWE.Timeout,
		MaxResponseBodySize:      10 * 1024 * 1024, // 10MB max response size
		DisablePathNormalizing:   true,             // Performance optimization
		NoDefaultUserAgentHeader: true,             // Reduce header overhead
		// Add critical timeout and connection management
		MaxConnDuration: 60 * time.Second, // Maximum connection lifetime
		ReadBufferSize:  4096,             // Optimize buffer sizes
		WriteBufferSize: 4096,
		// Add retry and error handling
		RetryIf: func(req *fasthttp.Request) bool {
			// Only retry on network errors, not on HTTP errors
			return false
		},
		// Add custom dialer with timeout
		Dial: func(addr string) (net.Conn, error) {
			dialer := &net.Dialer{
				Timeout:   10 * time.Second, // Connection establishment timeout
				KeepAlive: 30 * time.Second,
			}
			return dialer.Dial("tcp", addr)
		},
	}

	// Initialize Circuit Breaker - config-driven
	cbSettings := gobreaker.Settings{
		Name:        "TIMWE API Circuit Breaker",
		MaxRequests: uint32(max(1, cfg.Application.TIMWE.CBMaxRequests)),
		Interval:    cfg.Application.TIMWE.CBInterval,
		Timeout:     cfg.Application.TIMWE.CBTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			minReq := uint32(max(1, cfg.Application.TIMWE.CBMinRequests))
			if counts.Requests < minReq {
				return false
			}
			// Prefer explicit consecutive failures if set
			if cfg.Application.TIMWE.CBConsecutiveFailures > 0 {
				return counts.ConsecutiveFailures >= uint32(cfg.Application.TIMWE.CBConsecutiveFailures)
			}
			threshold := cfg.Application.TIMWE.CBFailureRateThreshold
			if threshold <= 0 {
				threshold = 0.8
			}
			failureRate := float64(counts.TotalFailures) / float64(max(1, int(counts.Requests)))
			return failureRate >= threshold
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("Circuit breaker %s state change: %v -> %v\n", name, from, to)
		},
	}
	circuitBreaker := gobreaker.NewTwoStepCircuitBreaker(cbSettings)

	// Initialize MSISDN validator with default config
	msisdnValidator := utils.NewMSISDNValidator(logger, userBaseRepository, nil)

	// Initialize network resilient client with default config
	networkClient := utils.NewNetworkResilientClient(logger, nil)

	s := &SubscriptionService{
		logger:             logger,
		repo:               repo,
		productRepo:        productRepo,
		UserBaseRepository: userBaseRepository,
		client:             client,
		networkClient:      networkClient,
		config:             cfg,
		circuitBreaker:     circuitBreaker,
		renewalService:     renewalService,
		msisdnValidator:    msisdnValidator,
	}
	if dbGetter, ok := repo.(repository.DBGetter); ok {
		s.tenantRouter = NewTenantProviderRouter(dbGetter.GetDB(), cfg, nil)
	}

	// Initialize bulkhead limiter for external calls
	limit := 200
	if limit <= 0 {
		limit = 200
	}
	s.bulkhead = make(chan struct{}, limit)

	// Start connection cleanup goroutine
	s.cleanupTicker = time.NewTicker(5 * time.Minute)
	go s.cleanupConnections()

	return s
}

// cleanupConnections periodically cleans up idle connections to prevent resource exhaustion
func (s *SubscriptionService) cleanupConnections() {
	for range s.cleanupTicker.C {
		// Close idle connections
		s.client.CloseIdleConnections()

		// Log connection pool status
		s.logger.Debug("Connection pool cleanup completed")
	}
}

func (s *SubscriptionService) acquireBulkhead() func() {
	s.bulkhead <- struct{}{}
	return func() { <-s.bulkhead }
}

func (s *SubscriptionService) getProductsCached(productIds []string) ([]*domain.Product, error) {
	key := "all"
	if len(productIds) > 0 {
		key = strings.Join(productIds, ",")
	}
	if cached, ok := s.productsCache.Load(key); ok {
		return cached.([]*domain.Product), nil
	}
	var (
		products []*domain.Product
		err      error
	)
	if len(productIds) == 0 {
		products, err = s.productRepo.GetProducts()
	} else {
		products, err = s.productRepo.GetProductsByIds(productIds)
	}
	if err != nil {
		return nil, err
	}
	s.productsCache.Store(key, products)
	return products, nil
}

func (s *SubscriptionService) ProcessOptin(req *domain.OptinRequest) error {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("PANIC RECOVERED in ProcessOptin",
				zap.Any("panic_value", r),
				zap.String("panic_type", fmt.Sprintf("%T", r)),
				zap.String("msisdn", req.Msisdn),
				zap.Strings("product_ids", req.ProductIds),
				zap.String("entry_channel", req.EntryChannel),
				zap.String("timestamp", time.Now().Format(time.RFC3339)),
			)

			// Use global panic handler if available
			if panicHandler := utils.GetGlobalPanicHandler(); panicHandler != nil {
				panicHandler.HandlePanic(r, context.Background())
			}
		}
	}()

	var products []*domain.Product
	var err error

	// Check if MSISDN is excluded (Staff, Premier, or Blacklisted) and exclude from processing
	isExcluded, err := s.UserBaseRepository.IsExcludedUser(req.Msisdn)
	if err != nil {
		s.logger.Error("Failed to check MSISDN type", zap.String("msisdn", req.Msisdn), zap.Error(err))
		return fmt.Errorf("failed to check MSISDN type for %s: %w", req.Msisdn, err)
	}

	if isExcluded {
		s.logger.Info("MSISDN is excluded type (Staff/Premier/Blacklisted), excluding from optin processing",
			zap.String("msisdn", req.Msisdn))
		return fmt.Errorf("MSISDN %s is excluded type and cannot be processed for optin", req.Msisdn)
	}

	// Fetch products from cache to avoid per-call DB lookups
	products, err = s.getProductsCached(req.ProductIds)
	if err != nil {
		s.logger.Error("Failed to fetch products", zap.Error(err))
		return fmt.Errorf("failed to fetch products: %w", err)
	}

	for _, product := range products {
		if err := s.processOptinForProduct(req, product); err != nil {
			return err
		}
	}
	return nil
}

// processOptinForProduct handles the optin process for a single product with retry logic
func (s *SubscriptionService) processOptinForProduct(req *domain.OptinRequest, product *domain.Product) error {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("PANIC RECOVERED in processOptinForProduct",
				zap.Any("panic_value", r),
				zap.String("panic_type", fmt.Sprintf("%T", r)),
				zap.String("msisdn", req.Msisdn),
				zap.String("product_id", product.ProductId),
				zap.String("product_name", product.Name),
				zap.String("timestamp", time.Now().Format(time.RFC3339)),
			)

			// Use global panic handler if available
			if panicHandler := utils.GetGlobalPanicHandler(); panicHandler != nil {
				panicHandler.HandlePanic(r, context.Background())
			}
		}
	}()

	// TODO: temporarily disable this check: Pre-validate MSISDN before making API calls
	// validationResult, err := s.msisdnValidator.ValidateMSISDN(context.Background(), req.Msisdn)
	// if err != nil {
	// 	s.logger.Error("MSISDN validation error",
	// 		zap.String("msisdn", req.Msisdn),
	// 		zap.Error(err))
	// 	return fmt.Errorf("MSISDN validation error: %w", err)
	// }

	// if !validationResult.IsValid {
	// 	s.logger.Warn("INVALID_MSISDN detected before API call - preventing external request",
	// 		zap.String("msisdn", req.Msisdn),
	// 		zap.String("reason", validationResult.ErrorReason),
	// 		zap.String("operator", validationResult.Operator))

	// 	// Return a domain error that mimics the TIMWE API response
	// 	return &domain.MTResponseError{
	// 		Code:    SubscriptionResultInvalidMsisdn,
	// 		Message: SubscriptionErrorInvalidMsisdn,
	// 		Details: map[string]interface{}{
	// 			"validationReason": validationResult.ErrorReason,
	// 			"operator":         validationResult.Operator,
	// 			"preventedAPICall": true,
	// 		},
	// 	}
	// }

	// // Use the formatted MSISDN for the API call
	// req.Msisdn = validationResult.FormattedMSISDN

	// s.logger.Info("MSISDN validation passed",
	// 	zap.String("msisdn", req.Msisdn),
	// 	zap.String("operator", validationResult.Operator))

	txId := uuid.New().String()
	productId, err := strconv.Atoi(product.ProductId)
	if err != nil {
		s.logger.Error("Failed to convert ProductId", zap.Error(err))
		return fmt.Errorf("failed to convert ProductId: %w", err)
	}

	reqDate := time.Now().Format(time.RFC3339)
	keyword := s.generateThreeLetterKeyword(product.Name)

	// First attempt with original entry channel
	mtReq := domain.MTRequest{
		ProductID:          productId,
		PricepointID:       product.PricePointId,
		UserIdentifier:     req.Msisdn,
		UserIdentifierType: "MSISDN",
		SubKeyword:         keyword,
		Context:            "Subscription",
		MCC:                s.getMCC(),
		MNC:                s.getMNC(),
		EntryChannel:       req.EntryChannel,
		LargeAccount:       product.ShortCode,
		MoTransactionUUID:  txId,
		SendDate:           reqDate,
		CampaignUrl:        "INTERNAL",
		Priority:           "NORMAL",
		Timezone:           "UTC",
	}

	realm := s.config.Application.TIMWE.Realm
	response, err := s.SendMT(mtReq, realm, req.EntryChannel)
	if err != nil {
		s.logger.Error("Error sending MT for msisdn", zap.String("msisdn", req.Msisdn), zap.Error(err))
		return err
	}

	// Check if we need to retry with SMS entry channel
	if s.shouldRetryWithSMS(response) {
		s.logger.Info("OPTIN_CONFIG_NOT_FOUND detected, retrying with SMS entry channel",
			zap.String("msisdn", req.Msisdn),
			zap.String("productId", product.ProductId),
			zap.String("originalChannel", req.EntryChannel))

		// Retry with SMS entry channel
		mtReq.EntryChannel = "SMS"
		response, err = s.SendMT(mtReq, realm, "SMS")
		if err != nil {
			s.logger.Error("Error sending MT retry with SMS for msisdn", zap.String("msisdn", req.Msisdn), zap.Error(err))
			return err
		}

		s.logger.Info("Retry with SMS completed",
			zap.String("msisdn", req.Msisdn),
			zap.String("productId", product.ProductId),
			zap.String("retryChannel", "SMS"),
			zap.String("responseCode", response.Code))
	}

	// After retry (if any), check if we still have OPTIN_CONFIG_NOT_FOUND
	if s.shouldRetryWithSMS(response) {
		s.logger.Error("OPTIN_CONFIG_NOT_FOUND still present after SMS retry, cannot proceed",
			zap.String("msisdn", req.Msisdn),
			zap.String("productId", product.ProductId),
			zap.String("finalChannel", mtReq.EntryChannel))
		return fmt.Errorf("OPTIN_CONFIG_NOT_FOUND persists after SMS retry for msisdn %s, product %s", req.Msisdn, product.ProductId)
	}

	// Now process the response normally
	if response.Code == ResponseCodeSuccess {
		transactionIdStr, err := s.getTransactionID(response)
		if err != nil {
			s.logger.Error("Failed to get transaction ID",
				zap.String("msisdn", req.Msisdn),
				zap.Error(err),
				zap.Any("responseData", response.ResponseData))
			return fmt.Errorf("failed to get transaction ID for msisdn %s: %w", req.Msisdn, err)
		}

		if s.isSubscriptionAlreadyActive(response) {
			s.logger.Info("User already has active subscription, skipping database save",
				zap.String("msisdn", req.Msisdn),
				zap.String("productId", product.ProductId))
			if err := s.HandleAlreadyActiveSubscription(req.Msisdn, product, req.EntryChannel); err != nil {
				s.logger.Error("Error handling already active subscription", zap.String("msisdn", req.Msisdn), zap.Error(err))
				return err
			}
			return nil
		}
		partnerRoleId, err := strconv.Atoi(s.config.Application.TIMWE.PartnerRoleID)
		if err != nil {
			s.logger.Error("Failed to convert PartnerRoleId", zap.Error(err))
			return fmt.Errorf("failed to convert PartnerRoleId for %s: %w", req.Msisdn, err)
		}

		if s.isSubscriptionWaitingForCharging(response) {
			s.logger.Info("User subscription is active and waiting for charging",
				zap.String("msisdn", req.Msisdn),
				zap.String("productId", product.ProductId))

			if err := s.HandleWaitingForChargingSubscription(req.Msisdn, product, req.EntryChannel, transactionIdStr, partnerRoleId, mtReq); err != nil {
				s.logger.Error("Error handling waiting for charging subscription", zap.String("msisdn", req.Msisdn), zap.Error(err))
				return err
			}
			return nil
		}

		subscriptionRequest := domain.MapMTRequestToSubscriptionRequest(mtReq, transactionIdStr, partnerRoleId, "INTERNAL", "INTERNAL")

		if err := s.repo.CreateSubscription(&subscriptionRequest); err != nil {
			s.logger.Error("Error saving subscription", zap.Error(err))
			return err
		}

		s.logger.Info("Subscription saved successfully",
			zap.String("msisdn", req.Msisdn),
			zap.String("transactionId", transactionIdStr),
			zap.String("productId", product.ProductId))
	} else {
		s.logger.Warn("MT request did not return SUCCESS code",
			zap.String("msisdn", req.Msisdn),
			zap.String("code", response.Code),
			zap.String("requestId", response.RequestID))
	}
	return nil
}

// shouldRetryWithSMS checks if the response indicates OPTIN_CONFIG_NOT_FOUND and should trigger SMS retry
func (s *SubscriptionService) shouldRetryWithSMS(response *domain.MTResponse) bool {
	// Check subscription result for OPTIN_CONFIG_NOT_FOUND
	if subscriptionResult, exists := response.ResponseData["subscriptionResult"]; exists && subscriptionResult != nil {
		if resultStr, ok := subscriptionResult.(string); ok {
			return resultStr == SubscriptionResultOptinConfigNotFound
		}
	}

	// Check subscription error for OPTIN_CONFIG_NOT_FOUND
	if subscriptionError, exists := response.ResponseData["subscriptionError"]; exists && subscriptionError != nil {
		if errorStr, ok := subscriptionError.(string); ok {
			return errorStr == SubscriptionErrorOptinConfigNotFound
		}
	}

	return false
}

// BackfillMsisdnsMissingSomeProducts fetches active MSISDNs that are missing at least one of the given products using windowing (start/end)
func (s *SubscriptionService) BackfillMsisdnsMissingSomeProducts(productIds []string, startIndex, endIndex int) ([]string, error) {
	if len(productIds) == 0 {
		return []string{}, nil
	}
	intIds := make([]int, 0, len(productIds))
	for _, pid := range productIds {
		v, err := strconv.Atoi(pid)
		if err != nil {
			s.logger.Error("invalid product id", zap.String("productId", pid), zap.Error(err))
			continue
		}
		intIds = append(intIds, v)
	}

	// Handle indexing logic based on requirements:
	// - If start_index is 0 and end_index is 0, select all subscriptions (no limit/offset)
	// - If start_index > 0 and end_index > 0, use windowing [startIndex, endIndex]
	// - If only end_index is provided, use [1, endIndex]

	offset := 0
	limit := 0

	if startIndex == 0 && endIndex == 0 {
		// Select all subscriptions - no limit or offset
		limit = 0
		offset = 0
	} else if startIndex > 0 && endIndex > 0 {
		// Use windowing with both start and end indices
		if endIndex >= startIndex {
			offset = startIndex - 1 // Convert to 0-based offset
			limit = (endIndex - startIndex) + 1
		} else {
			// Invalid range, return empty
			return []string{}, nil
		}
	} else if startIndex == 0 && endIndex > 0 {
		// Only end_index provided, use [1, endIndex]
		offset = 0
		limit = endIndex
	} else if startIndex > 0 && endIndex == 0 {
		// Only start_index provided, use [startIndex, ∞]
		offset = startIndex - 1
		limit = 0 // No limit
	}

	s.logger.Info("BackfillMsisdnsMissingSomeProducts called",
		zap.Ints("productIds", intIds),
		zap.Int("startIndex", startIndex),
		zap.Int("endIndex", endIndex),
		zap.Int("offset", offset),
		zap.Int("limit", limit),
	)

	result, err := s.repo.FetchActiveMsisdnsMissingSomeProducts(intIds, offset, limit)
	if err != nil {
		s.logger.Error("Failed to fetch msisdns missing some products", zap.Error(err))
		return nil, err
	}

	s.logger.Info("BackfillMsisdnsMissingSomeProducts result",
		zap.Int("resultCount", len(result)),
		zap.Ints("productIds", intIds),
		zap.Int("offset", offset),
		zap.Int("limit", limit),
	)

	return result, nil
}

// BackfillMsisdnsWithProducts fetches active MSISDNs that already have specified products using windowing (start/end)
func (s *SubscriptionService) BackfillMsisdnsWithProducts(productIds []string, startIndex, endIndex int) ([]string, error) {
	if len(productIds) == 0 {
		return []string{}, nil
	}
	intIds := make([]int, 0, len(productIds))
	for _, pid := range productIds {
		v, err := strconv.Atoi(pid)
		if err != nil {
			s.logger.Error("invalid product id", zap.String("productId", pid), zap.Error(err))
			continue
		}
		intIds = append(intIds, v)
	}

	// Handle indexing logic based on requirements:
	// - If start_index is 0 and end_index is 0, select all subscriptions (no limit/offset)
	// - If start_index > 0 and end_index > 0, use windowing [startIndex, endIndex]
	// - If only end_index is provided, use [1, endIndex]

	offset := 0
	limit := 0

	if startIndex == 0 && endIndex == 0 {
		// Select all subscriptions - no limit or offset
		limit = 0
		offset = 0
	} else if startIndex > 0 && endIndex > 0 {
		// Use windowing with both start and end indices
		if endIndex >= startIndex {
			offset = startIndex - 1 // Convert to 0-based offset
			limit = (endIndex - startIndex) + 1
		} else {
			// Invalid range, return empty
			return []string{}, nil
		}
	} else if startIndex == 0 && endIndex > 0 {
		// Only end_index provided, use [1, endIndex]
		offset = 0
		limit = endIndex
	} else if startIndex > 0 && endIndex == 0 {
		// Only start_index provided, use [startIndex, ∞]
		offset = startIndex - 1
		limit = 0 // No limit
	}

	return s.repo.FetchActiveMsisdnsWithProductsWindow(intIds, offset, limit)
}

// ResubscribeUser performs unsubscribe then re-subscribe for the provided MSISDN across given products.
func (s *SubscriptionService) ResubscribeUser(msisdn string, entryChannel string, productIds []string) error {
	// Build an opt-out request for each product, then re-opt-in using ProcessOptin
	products, err := s.getProductsCached(productIds)
	if err != nil {
		return fmt.Errorf("failed to fetch products: %w", err)
	}
	for _, product := range products {
		pid, err := strconv.Atoi(product.ProductId)
		if err != nil {
			s.logger.Error("Failed to convert ProductId", zap.Error(err))
			continue
		}
		// Construct minimal UnsubscriptionRequest. Some fields may be optional/downstream ignored.
		optoutReq := domain.UnsubscriptionRequest{
			UserIdentifier:        msisdn,
			UserIdentifierType:    "MSISDN",
			ProductId:             pid,
			Mcc:                   stringPtr("620"),
			Mnc:                   stringPtr("03"),
			EntryChannel:          stringPtr(entryChannel),
			LargeAccount:          stringPtr(product.ShortCode),
			SubKeyword:            stringPtr("STOP " + s.generateThreeLetterKeyword(product.Name)),
			TrackingId:            stringPtr(uuid.New().String()),
			ClientIp:              stringPtr("INTERNAL"),
			ControlKeyword:        "STOP",
			ControlServiceKeyword: "STOP",
			SubId:                 0,
			CancelReason:          0,
			CancelSource:          0,
		}
		if _, err := s.SendOptout(optoutReq, s.config.Application.TIMWE.Realm); err != nil {
			s.logger.Error("Failed to optout before resubscribe", zap.String("msisdn", msisdn), zap.String("productId", product.ProductId), zap.Error(err))
			// continue to next product rather than aborting all
			continue
		}

		// Now opt-in again for this single product
		optinReq := &domain.OptinRequest{
			Telco:        "",
			EntryChannel: entryChannel,
			Msisdn:       msisdn,
			ProductIds:   []string{product.ProductId},
		}
		if err := s.ProcessOptin(optinReq); err != nil {
			s.logger.Error("Failed to optin after optout", zap.String("msisdn", msisdn), zap.String("productId", product.ProductId), zap.Error(err))
			// continue to next product
			continue
		}
	}
	return nil
}

// SendMT sends the MT request guarded by circuit breaker (acquire bulkhead)
func (s *SubscriptionService) SendMT(reqData domain.MTRequest, realm, channel string) (*domain.MTResponse, error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("PANIC RECOVERED in SendMT",
				zap.Any("panic_value", r),
				zap.String("panic_type", fmt.Sprintf("%T", r)),
				zap.String("msisdn", reqData.UserIdentifier),
				zap.Int("product_id", reqData.ProductID),
				zap.String("realm", realm),
				zap.String("channel", channel),
				zap.String("timestamp", time.Now().Format(time.RFC3339)),
			)

			// Use global panic handler if available
			if panicHandler := utils.GetGlobalPanicHandler(); panicHandler != nil {
				panicHandler.HandlePanic(r, context.Background())
			}
		}
	}()

	release := s.acquireBulkhead()
	defer release()

	providerCfg, err := s.providerConfigOrLegacy(context.Background(), ChannelOperationMT, reqData.TenantRoute)
	if err != nil {
		return nil, err
	}
	reqData.TenantRoute = canonicalTenantRoute(reqData.TenantRoute, providerCfg)
	authKey, err := providerCfg.AuthKey()
	if err != nil {
		s.logger.Error("failed to resolve auth key", zap.Error(err))
		return nil, fmt.Errorf("failed to resolve auth key: %w", err)
	}

	// Build URL and request body
	url := fmt.Sprintf("%s/subscription/optin/%s", providerCfg.BaseURL, providerCfg.PartnerRoleID)
	payload, err := s.buildTIMWEOptinPayload(reqData)
	if err != nil {
		s.logger.Error("Failed to normalize optin payload", zap.Error(err))
		return nil, err
	}

	requestBody, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("Failed to marshal request data", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal request data: %v", err)
	}

	// TwoStep: decide success based on error classification
	done, err := s.circuitBreaker.Allow()
	if err != nil {
		s.logger.Error("Circuit breaker open for SendMT", zap.Error(err))
		return nil, err
	}

	resp, callErr := s.sendMTWithRetry(reqData, url, providerCfg.APIKey, authKey, requestBody, 3)
	success := callErr == nil || s.isNonBreakerError(callErr)
	done(success)
	if callErr != nil {
		if success {
			// propagate domain error but don't count as breaker failure
			return nil, callErr
		}
		s.logger.Error("Circuit breaker classified failure in SendMT", zap.Error(callErr))
		return nil, callErr
	}

	// Check if we need to retry with SMS entry channel for OPTIN_CONFIG_NOT_FOUND
	if s.shouldRetryWithSMS(resp) {
		s.logger.Info("OPTIN_CONFIG_NOT_FOUND detected in SendMT, retrying with SMS entry channel",
			zap.String("msisdn", reqData.UserIdentifier),
			zap.String("originalChannel", channel))

		// Retry with SMS entry channel
		mtReqCopy := reqData
		mtReqCopy.EntryChannel = "SMS"
		payload, err = s.buildTIMWEOptinPayload(mtReqCopy)
		if err != nil {
			s.logger.Error("Failed to normalize retry optin payload", zap.Error(err))
			return nil, err
		}

		// Re-marshal the updated request
		requestBody, err = json.Marshal(payload)
		if err != nil {
			s.logger.Error("Failed to marshal retry request data", zap.Error(err))
			return nil, fmt.Errorf("failed to marshal retry request data: %v", err)
		}

		resp, callErr = s.sendMTWithRetry(mtReqCopy, url, providerCfg.APIKey, authKey, requestBody, 3)
		if callErr != nil {
			s.logger.Error("Error sending MT retry with SMS", zap.String("msisdn", reqData.UserIdentifier), zap.Error(callErr))
			return nil, callErr
		}

		s.logger.Info("SendMT retry with SMS completed",
			zap.String("msisdn", reqData.UserIdentifier),
			zap.String("retryChannel", "SMS"),
			zap.String("responseCode", resp.Code))
	}

	return resp, nil
}

// Classify errors that should NOT count toward breaker failures
func (s *SubscriptionService) isNonBreakerError(err error) bool {
	// Treat domain-level MTResponseError and 4xx as non-breaker failures
	if _, ok := err.(*domain.MTResponseError); ok {
		return true
	}
	// Additionally, unwrap specific HTTP status-based errors we generate
	errStr := err.Error()
	// Non-200 status errors include the code; skip breaker count for 4xx
	if strings.Contains(errStr, "status code:") {
		for _, code := range []string{" 400", " 401", " 403", " 404", " 409", " 422"} {
			if strings.Contains(errStr, code) {
				return true
			}
		}
	}
	return false
}

// sendMTWithRetry handles the actual MT request with retry logic for INTERNAL_ERROR
func (s *SubscriptionService) sendMTWithRetry(reqData domain.MTRequest, url, apiKey, authKey string, requestBody []byte, maxRetries int) (*domain.MTResponse, error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("PANIC RECOVERED in sendMTWithRetry",
				zap.Any("panic_value", r),
				zap.String("panic_type", fmt.Sprintf("%T", r)),
				zap.String("msisdn", reqData.UserIdentifier),
				zap.Int("product_id", reqData.ProductID),
				zap.String("url", url),
				zap.Int("max_retries", maxRetries),
				zap.String("timestamp", time.Now().Format(time.RFC3339)),
			)

			// Use global panic handler if available
			if panicHandler := utils.GetGlobalPanicHandler(); panicHandler != nil {
				panicHandler.HandlePanic(r, context.Background())
			}
		}
	}()

	baseDelay := 200 * time.Millisecond

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create context with timeout for this request
		ctx, cancel := context.WithTimeout(context.Background(), s.config.Application.TIMWE.Timeout)

		req := fasthttp.AcquireRequest()
		res := fasthttp.AcquireResponse()
		externalTxID := uuid.New().String()

		// Ensure cleanup happens regardless of how we exit
		defer func() {
			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(res)
		}()

		// Set up request
		req.SetRequestURI(url)
		req.Header.SetMethod("POST")
		req.Header.Set("apikey", apiKey)
		req.Header.Set("authentication", authKey)
		req.Header.Set("external-tx-id", externalTxID)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "*/*")
		req.SetBody(requestBody)

		// Send request with context timeout
		requestDone := make(chan error, 1)
		go func() {
			select {
			case requestDone <- s.client.Do(req, res):
			case <-ctx.Done():
				// Context was cancelled, don't block
				select {
				case requestDone <- ctx.Err():
				default:
				}
			}
		}()

		// Wait for request completion or timeout
		select {
		case err := <-requestDone:
			// Request completed
			if err != nil {
				s.logger.Warn("Failed to send request",
					zap.Int("attempt", attempt),
					zap.String("msisdn", reqData.UserIdentifier),
					zap.Error(err))

				if attempt == maxRetries {
					return nil, fmt.Errorf("failed to subscribe user after %d attempts: %v", maxRetries, err)
				}

				// Exponential backoff for network errors
				delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
				time.Sleep(delay)
				continue
			}
		case <-ctx.Done():
			// Context timeout or cancellation
			cancel()

			if attempt == maxRetries {
				return nil, fmt.Errorf("request timeout after %d attempts: %v", maxRetries, ctx.Err())
			}

			s.logger.Warn("Request timeout, retrying",
				zap.Int("attempt", attempt),
				zap.String("msisdn", reqData.UserIdentifier),
				zap.Error(ctx.Err()))

			// Exponential backoff for timeout errors
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			time.Sleep(delay)
			continue
		}

		// Clean up context
		cancel()

		// Check HTTP status code
		if res.StatusCode() != fasthttp.StatusOK {
			s.logger.Error("MT request failed with non-200 status",
				zap.Int("attempt", attempt),
				zap.Int("statusCode", res.StatusCode()),
				zap.String("responseBody", string(res.Body())),
				zap.String("msisdn", reqData.UserIdentifier),
				zap.String("url", url))

			if attempt == maxRetries {
				return nil, fmt.Errorf("subscription request failed with status code: %d", res.StatusCode())
			}

			// Exponential backoff for HTTP errors
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			time.Sleep(delay)
			continue
		}

		// Parse response
		var mtResponse domain.MTResponse
		if err := json.Unmarshal(res.Body(), &mtResponse); err != nil {
			s.logger.Error("Failed to parse response",
				zap.Int("attempt", attempt),
				zap.String("msisdn", reqData.UserIdentifier),
				zap.Error(err))

			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to parse response: %v", err)
			}

			// Exponential backoff for parsing errors
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			time.Sleep(delay)
			continue
		}

		// Log the raw response for debugging
		s.logger.Info("MT API response received",
			zap.Int("attempt", attempt),
			zap.String("msisdn", reqData.UserIdentifier),
			zap.String("code", mtResponse.Code),
			zap.Bool("inError", mtResponse.InError),
			zap.String("requestId", mtResponse.RequestID),
			zap.Any("responseData", mtResponse.ResponseData))

		// Check for INTERNAL_ERROR and retry if needed
		if mtResponse.Code == ResponseCodeInternalError {
			s.logger.Warn("MT request failed with internal error, retrying",
				zap.Int("attempt", attempt),
				zap.String("msisdn", reqData.UserIdentifier),
				zap.String("requestId", mtResponse.RequestID))

			if attempt == maxRetries {
				s.logger.Error("MT request failed with internal error after all retries",
					zap.String("msisdn", reqData.UserIdentifier),
					zap.String("requestId", mtResponse.RequestID))
				return nil, fmt.Errorf("MT request failed with internal error after %d attempts: requestId=%s", maxRetries, mtResponse.RequestID)
			}

			// Exponential backoff for INTERNAL_ERROR
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			s.logger.Info("Retrying MT request after INTERNAL_ERROR",
				zap.Int("attempt", attempt+1),
				zap.Duration("delay", delay),
				zap.String("msisdn", reqData.UserIdentifier))
			time.Sleep(delay)
			continue
		}

		// Validate and handle different response scenarios
		if err := s.validateMTResponse(&mtResponse, reqData); err != nil {
			s.logger.Error("MT response validation failed",
				zap.Int("attempt", attempt),
				zap.String("msisdn", reqData.UserIdentifier),
				zap.Error(err))

			return nil, err
		}

		// Handle different response codes
		switch mtResponse.Code {
		case ResponseCodeSuccess:
			s.logger.Info("MT request successful",
				zap.Int("attempt", attempt),
				zap.String("msisdn", reqData.UserIdentifier),
				zap.String("requestId", mtResponse.RequestID))
		default:
			s.logger.Warn("MT request returned unexpected code",
				zap.Int("attempt", attempt),
				zap.String("msisdn", reqData.UserIdentifier),
				zap.String("code", mtResponse.Code),
				zap.String("requestId", mtResponse.RequestID))
		}

		return &mtResponse, nil
	}

	return nil, fmt.Errorf("unexpected end of retry loop")
}

// validateMTResponse validates the MT response and handles different subscription results
func (s *SubscriptionService) validateMTResponse(response *domain.MTResponse, mtReq domain.MTRequest) error {
	// Check if response data exists
	if response.ResponseData == nil {
		return fmt.Errorf("response data is nil")
	}

	// Detect and log INVALID_MSISDN responses (non-blocking)
	partnerRoleID, err := strconv.Atoi(s.config.Application.TIMWE.PartnerRoleID)
	if err != nil {
		s.logger.Error("Failed to parse partner role ID", zap.Int("partnerRoleID", partnerRoleID), zap.Error(err))
		return fmt.Errorf("invalid partner role ID: %w", err)
	}

	s.detectAndLogInvalidMSISDN(response, mtReq, partnerRoleID)

	// Handle BLACKLISTED responses by adding user to blacklist and removing subscriptions
	if response.Code == ResponseCodeBlacklisted {
		s.logger.Warn("BLACKLISTED response received, adding user to blacklist and removing subscriptions",
			zap.String("msisdn", mtReq.UserIdentifier),
			zap.String("requestId", response.RequestID))

		// Enhanced: Process blacklisted user handling asynchronously for better performance
		go s.handleBlacklistedUserEnhanced(mtReq.UserIdentifier, mtReq.ProductID, response.RequestID, partnerRoleID, response)

		// Return error to indicate the operation failed
		return &domain.MTResponseError{
			Code:    response.Code,
			Message: "User is blacklisted",
			Details: response.ResponseData,
		}
	}

	// First, check if the main response indicates an error
	if response.InError {
		s.logger.Error("MT response indicates error",
			zap.String("code", response.Code),
			zap.String("message", response.Message))
		return &domain.MTResponseError{
			Code:    response.Code,
			Message: response.Message,
			Details: response.ResponseData,
		}
	}

	// Check if the response code indicates an error (even if inError is false)
	if response.Code != ResponseCodeSuccess {
		s.logger.Error("MT response code indicates error",
			zap.String("code", response.Code),
			zap.String("message", response.Message))
		return &domain.MTResponseError{
			Code:    response.Code,
			Message: response.Message,
			Details: response.ResponseData,
		}
	}

	// Check subscription result
	if subscriptionResult, exists := response.ResponseData["subscriptionResult"]; exists && subscriptionResult != nil {
		if resultStr, ok := subscriptionResult.(string); ok && resultStr != SubscriptionResultNull {
			s.logger.Info("Subscription result", zap.String("result", resultStr))

			// Success codes for opt-in and opt-out flows
			if resultStr == SubscriptionResultOptinAlreadyActive ||
				resultStr == SubscriptionResultOptinActiveWaitCharging ||
				resultStr == SubscriptionResultOptinPreactiveWaitConf ||
				resultStr == SubscriptionResultOptoutCanceledOK ||
				resultStr == SubscriptionResultOptoutNoSub {
				return nil
			}

			// Special case: OPTIN_CONFIG_NOT_FOUND should be allowed to pass through
			// so that the retry logic can handle it with SMS entry channel
			if resultStr == SubscriptionResultOptinConfigNotFound {
				s.logger.Info("OPTIN_CONFIG_NOT_FOUND detected, allowing to pass through for retry logic",
					zap.String("subscriptionResult", resultStr))
				return nil
			}

			// All other subscription results are treated as errors
			s.logger.Error("Subscription result indicates error",
				zap.String("subscriptionResult", resultStr))
			return &domain.MTResponseError{
				Code:    response.Code,
				Message: fmt.Sprintf("subscription error: %s", resultStr),
				Details: response.ResponseData,
			}
		}
	}

	// Extract subscription error if available
	if subscriptionError, exists := response.ResponseData["subscriptionError"]; exists && subscriptionError != nil {
		if errorStr, ok := subscriptionError.(string); ok && errorStr != SubscriptionResultNull {
			s.logger.Warn("Subscription error received", zap.String("error", errorStr))
			// Don't return error for informational messages
			if errorStr != SubscriptionErrorAlreadyActive && errorStr != SubscriptionErrorActiveWaitCharging && errorStr != SubscriptionErrorOptoutOneSuccess && errorStr != SubscriptionErrorOptoutNonExistent {
				return &domain.MTResponseError{
					Code:    response.Code,
					Message: fmt.Sprintf("subscription error: %s", errorStr),
					Details: response.ResponseData,
				}
			}
		}
	}

	// Validate transaction ID exists for successful responses
	if response.Code == ResponseCodeSuccess {
		if _, err := s.getTransactionID(response); err != nil {
			return &domain.MTResponseError{
				Code:    response.Code,
				Message: "transaction ID missing from successful response",
				Details: response.ResponseData,
			}
		}
	}

	return nil
}

func (s *SubscriptionService) RequestCharge(reqData domain.ChargeRequest) (*domain.ChargeResponse, error) {
	providerCfg, err := s.providerConfigOrLegacy(context.Background(), ChannelOperationCharge, reqData.TenantRoute)
	if err != nil {
		return nil, err
	}
	reqData.TenantRoute = canonicalTenantRoute(reqData.TenantRoute, providerCfg)
	authKey, err := providerCfg.AuthKey()
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/%s/charge/dob/%s", providerCfg.BaseURL, providerCfg.Realm, providerCfg.PartnerRoleID)

	done, err := s.circuitBreaker.Allow()
	if err != nil {
		return nil, err
	}

	resp, callErr := s.sendChargeRequest(reqData, url, providerCfg.APIKey, authKey)
	success := callErr == nil || s.isNonBreakerError(callErr)
	done(success)
	if callErr != nil {
		if success {
			return nil, callErr
		}
		return nil, callErr
	}
	return resp, nil
}

// Send charge request with configurable time-bounded exponential backoff and jitter
func (s *SubscriptionService) sendChargeRequest(reqData domain.ChargeRequest, url, apiKey, authKey string) (*domain.ChargeResponse, error) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	// Set request URL and headers
	req.SetRequestURI(url)
	req.Header.SetMethod("POST")
	req.Header.Set("apikey", apiKey)
	req.Header.Set("authentication", authKey)
	req.Header.Set("Content-Type", "application/json")

	// Set request body
	requestBody, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request data: %v", err)
	}
	req.SetBody(requestBody)

	// Exponential backoff settings from config
	maxDuration := s.config.Application.TIMWE.ChargeRetryMaxDuration
	baseDelay := s.config.Application.TIMWE.ChargeRetryBaseDelay
	maxDelay := s.config.Application.TIMWE.ChargeRetryMaxDelay
	if baseDelay <= 0 {
		baseDelay = 200 * time.Millisecond
	}
	if maxDuration <= 0 {
		maxDuration = 2 * time.Minute
	}
	if maxDelay <= 0 {
		maxDelay = 5 * time.Second
	}

	start := time.Now()
	attempt := 0
	for time.Since(start) < maxDuration {
		attempt++
		err = s.client.Do(req, res)
		if err == nil && res.StatusCode() == fasthttp.StatusOK {
			var chargeResponse domain.ChargeResponse
			if err := json.Unmarshal(res.Body(), &chargeResponse); err != nil {
				return nil, fmt.Errorf("failed to parse response: %v", err)
			}
			return &chargeResponse, nil
		}

		// compute exponential delay with cap and simple jitter
		exp := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
		if exp > maxDelay {
			exp = maxDelay
		}
		jitter := time.Duration(rand.Int63n(int64(exp / 5))) // up to 20% jitter
		delay := exp + jitter

		s.logger.Error("Charge request failed; retrying", zap.Int("attempt", attempt), zap.Duration("delay", delay), zap.Error(err))
		time.Sleep(delay)
	}

	return nil, fmt.Errorf("request failed after %d attempts over %s: %v", attempt, maxDuration, err)
}

func (s *SubscriptionService) FetchProducts() ([]*domain.Product, error) {
	return s.productRepo.GetProducts()
}

// Helper method to safely extract string value from response data
func (s *SubscriptionService) extractStringFromResponse(responseData map[string]interface{}, key string) (string, bool) {
	if value, exists := responseData[key]; exists && value != nil {
		if strValue, ok := value.(string); ok {
			return strValue, true
		}
	}
	return "", false
}

// Helper method to check if subscription is already active
func (s *SubscriptionService) isSubscriptionAlreadyActive(response *domain.MTResponse) bool {
	if subscriptionResult, exists := response.ResponseData["subscriptionResult"]; exists && subscriptionResult != nil {
		if resultStr, ok := subscriptionResult.(string); ok {
			// Only these two results indicate an active subscription
			return resultStr == SubscriptionResultOptinAlreadyActive || resultStr == SubscriptionResultOptinActiveWaitCharging
		}
	}
	return false
}

// Helper method to check if subscription is waiting for charging
func (s *SubscriptionService) isSubscriptionWaitingForCharging(response *domain.MTResponse) bool {
	if subscriptionResult, exists := response.ResponseData["subscriptionResult"]; exists && subscriptionResult != nil {
		if resultStr, ok := subscriptionResult.(string); ok {
			return resultStr == SubscriptionResultOptinActiveWaitCharging
		}
	}
	return false
}

// Helper method to get transaction ID safely
func (s *SubscriptionService) getTransactionID(response *domain.MTResponse) (string, error) {
	transactionId, exists := response.ResponseData["transactionId"]
	if !exists || transactionId == nil {
		return "", fmt.Errorf("transaction ID missing from response")
	}

	transactionIdStr, ok := transactionId.(string)
	if !ok {
		return "", fmt.Errorf("transaction ID is not a string: %v", transactionId)
	}

	return transactionIdStr, nil
}

// Helper method to detect and log INVALID_MSISDN responses
func (s *SubscriptionService) detectAndLogInvalidMSISDN(response *domain.MTResponse, mtReq domain.MTRequest, partnerId int) {
	// Check if the response indicates INVALID_MSISDN
	isInvalidMSISDN := false
	subscriptionResult := ""
	subscriptionError := ""

	// Check main response code
	if response.Code == SubscriptionResultInvalidMsisdn {
		isInvalidMSISDN = true
	}

	// Check subscription result
	if result, exists := response.ResponseData["subscriptionResult"]; exists && result != nil {
		if resultStr, ok := result.(string); ok {
			subscriptionResult = resultStr
			if resultStr == SubscriptionResultInvalidMsisdn {
				isInvalidMSISDN = true
			}
		}
	}

	// Check subscription error
	if error, exists := response.ResponseData["subscriptionError"]; exists && error != nil {
		if errorStr, ok := error.(string); ok {
			subscriptionError = errorStr
			if errorStr == SubscriptionErrorInvalidMsisdn {
				isInvalidMSISDN = true
			}
		}
	}

	// If INVALID_MSISDN is detected, log it and clean up subscriptions
	if isInvalidMSISDN {
		s.logger.Warn("INVALID_MSISDN detected, logging for reference and cleaning up subscriptions",
			zap.String("msisdn", mtReq.UserIdentifier),
			zap.String("responseCode", response.Code),
			zap.String("subscriptionResult", subscriptionResult),
			zap.String("subscriptionError", subscriptionError))

		// Create log entry
		logEntry := &domain.InvalidMSISDNLog{
			MSISDN:             mtReq.UserIdentifier,
			ProductID:          &mtReq.ProductID,
			PricepointID:       &mtReq.PricepointID,
			PartnerRoleID:      &partnerId,
			EntryChannel:       mtReq.EntryChannel,
			RequestID:          response.RequestID,
			ResponseCode:       response.Code,
			ResponseMessage:    response.Message,
			SubscriptionResult: subscriptionResult,
			SubscriptionError:  subscriptionError,
			ExternalTxID:       mtReq.MoTransactionUUID,
			CreatedAt:          time.Now(),
		}

		// Save to database (non-blocking)
		if err := s.repo.CreateInvalidMSISDNLog(logEntry); err != nil {
			s.logger.Error("Failed to save invalid MSISDN log",
				zap.String("msisdn", mtReq.UserIdentifier),
				zap.Error(err))
		}

		// Enhanced: Process cleanup asynchronously for better performance
		go s.handleInvalidMSISDNCleanup(mtReq.UserIdentifier, mtReq.ProductID, response.RequestID)
	}
}

// handleInvalidMSISDNCleanup handles the cleanup of invalid MSISDN subscriptions asynchronously
func (s *SubscriptionService) handleInvalidMSISDNCleanup(msisdn string, productId int, requestID string) {
	startTime := time.Now()
	success := false
	defer func() {
		// Track metrics
		duration := time.Since(startTime)
		if success {
			s.logger.Info("Invalid MSISDN cleanup completed successfully",
				zap.String("msisdn", msisdn),
				zap.Int("productId", productId),
				zap.String("requestId", requestID),
				zap.Duration("duration", duration))
		} else {
			s.logger.Error("Invalid MSISDN cleanup failed",
				zap.String("msisdn", msisdn),
				zap.Int("productId", productId),
				zap.String("requestId", requestID),
				zap.Duration("duration", duration))
		}
	}()

	// Step 1: Check if any subscriptions exist for this MSISDN (product-independent)
	hasSubscriptions, err := s.hasSubscription(msisdn)
	if err != nil {
		s.logger.Error("Failed to check subscription existence for cleanup",
			zap.String("msisdn", msisdn),
			zap.Error(err))
		return
	}

	if !hasSubscriptions {
		s.logger.Debug("No subscriptions found for invalid MSISDN, skipping cleanup",
			zap.String("msisdn", msisdn))
		success = true
		return
	}

	// Step 2: Attempt to delete all subscriptions for this MSISDN with retry logic
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = s.repo.DeleteSubscriptionRecord(msisdn)
		if err == nil {
			success = true
			s.logger.Info("Successfully deleted all subscription records for invalid MSISDN",
				zap.String("msisdn", msisdn),
				zap.Int("attempt", attempt))
			break
		}

		// Log retry attempt
		s.logger.Warn("Failed to delete subscription records for invalid MSISDN, retrying",
			zap.String("msisdn", msisdn),
			zap.Int("attempt", attempt),
			zap.Int("maxRetries", maxRetries),
			zap.Error(err))

		// Wait before retry with exponential backoff
		if attempt < maxRetries {
			backoffDuration := time.Duration(attempt*attempt) * 100 * time.Millisecond
			time.Sleep(backoffDuration)
		}
	}

	// If all retries failed, log the final error
	if !success {
		s.logger.Error("Failed to delete subscription records for invalid MSISDN after all retries",
			zap.String("msisdn", msisdn),
			zap.Int("maxRetries", maxRetries),
			zap.Error(err))
	}
}

// hasSubscription checks if any subscriptions exist for the given MSISDN (product-independent)
func (s *SubscriptionService) hasSubscription(msisdn string) (bool, error) {
	// Use the new repository method to check for any subscriptions
	// This is product-independent since we want to clean up ALL subscriptions for an invalid MSISDN
	hasSubscriptions, err := s.repo.HasAnySubscription(msisdn)
	if err != nil {
		return false, fmt.Errorf("failed to check subscription existence: %w", err)
	}
	return hasSubscriptions, nil
}

// BatchHandleInvalidMSISDNs processes multiple INVALID_MSISDN responses efficiently
func (s *SubscriptionService) BatchHandleInvalidMSISDNs(responses []*domain.MTResponse, requests []domain.MTRequest, partnerId int) {
	if len(responses) == 0 || len(requests) == 0 {
		return
	}

	s.logger.Info("Starting batch processing of INVALID_MSISDN responses",
		zap.Int("responseCount", len(responses)),
		zap.Int("requestCount", len(requests)))

	// Group invalid MSISDNs for batch processing
	var invalidMSISDNs []string
	var invalidMSISDNLogs []*domain.InvalidMSISDNLog
	var cleanupTasks []struct {
		msisdn    string
		productId int
		requestID string
	}

	for i, response := range responses {
		if i >= len(requests) {
			break
		}
		mtReq := requests[i]

		// Check if this response indicates INVALID_MSISDN
		isInvalidMSISDN := s.isInvalidMSISDNResponse(response)

		if isInvalidMSISDN {
			invalidMSISDN := mtReq.UserIdentifier
			invalidMSISDNs = append(invalidMSISDNs, invalidMSISDN)

			// Create log entry
			logEntry := &domain.InvalidMSISDNLog{
				MSISDN:             mtReq.UserIdentifier,
				ProductID:          &mtReq.ProductID,
				PricepointID:       &mtReq.PricepointID,
				PartnerRoleID:      &partnerId,
				EntryChannel:       mtReq.EntryChannel,
				RequestID:          response.RequestID,
				ResponseCode:       response.Code,
				ResponseMessage:    response.Message,
				SubscriptionResult: s.extractSubscriptionResult(response),
				SubscriptionError:  s.extractSubscriptionError(response),
				ExternalTxID:       mtReq.MoTransactionUUID,
				CreatedAt:          time.Now(),
			}
			invalidMSISDNLogs = append(invalidMSISDNLogs, logEntry)

			// Add to cleanup tasks
			cleanupTasks = append(cleanupTasks, struct {
				msisdn    string
				productId int
				requestID string
			}{
				msisdn:    mtReq.UserIdentifier,
				productId: mtReq.ProductID,
				requestID: response.RequestID,
			})
		}
	}

	if len(invalidMSISDNs) == 0 {
		s.logger.Debug("No INVALID_MSISDN responses found in batch")
		return
	}

	s.logger.Info("Found INVALID_MSISDN responses in batch, processing cleanup",
		zap.Int("invalidCount", len(invalidMSISDNs)),
		zap.Strings("invalidMSISDNs", invalidMSISDNs))

	// Step 1: Batch log all invalid MSISDNs
	s.batchCreateInvalidMSISDNLogs(invalidMSISDNLogs)

	// Step 2: Batch cleanup subscriptions
	s.batchCleanupInvalidMSISDNSubscriptions(cleanupTasks)
}

// isInvalidMSISDNResponse checks if a response indicates INVALID_MSISDN
func (s *SubscriptionService) isInvalidMSISDNResponse(response *domain.MTResponse) bool {
	// Check main response code
	if response.Code == SubscriptionResultInvalidMsisdn {
		return true
	}

	// Check subscription result
	if result, exists := response.ResponseData["subscriptionResult"]; exists && result != nil {
		if resultStr, ok := result.(string); ok {
			if resultStr == SubscriptionResultInvalidMsisdn {
				return true
			}
		}
	}

	// Check subscription error
	if error, exists := response.ResponseData["subscriptionError"]; exists && error != nil {
		if errorStr, ok := error.(string); ok {
			if errorStr == SubscriptionErrorInvalidMsisdn {
				return true
			}
		}
	}

	return false
}

// extractSubscriptionResult safely extracts subscription result from response
func (s *SubscriptionService) extractSubscriptionResult(response *domain.MTResponse) string {
	if result, exists := response.ResponseData["subscriptionResult"]; exists && result != nil {
		if resultStr, ok := result.(string); ok {
			return resultStr
		}
	}
	return ""
}

// extractSubscriptionError safely extracts subscription error from response
func (s *SubscriptionService) extractSubscriptionError(response *domain.MTResponse) string {
	if error, exists := response.ResponseData["subscriptionError"]; exists && error != nil {
		if errorStr, ok := error.(string); ok {
			return errorStr
		}
	}
	return ""
}

// batchCreateInvalidMSISDNLogs creates multiple invalid MSISDN logs efficiently
func (s *SubscriptionService) batchCreateInvalidMSISDNLogs(logs []*domain.InvalidMSISDNLog) {
	if len(logs) == 0 {
		return
	}

	// Process in batches to avoid overwhelming the database
	batchSize := 100
	for i := 0; i < len(logs); i += batchSize {
		end := i + batchSize
		if end > len(logs) {
			end = len(logs)
		}
		batch := logs[i:end]

		// Process batch concurrently
		var wg sync.WaitGroup
		for _, logEntry := range batch {
			wg.Add(1)
			go func(log *domain.InvalidMSISDNLog) {
				defer wg.Done()
				if err := s.repo.CreateInvalidMSISDNLog(log); err != nil {
					s.logger.Error("Failed to save invalid MSISDN log in batch",
						zap.String("msisdn", log.MSISDN),
						zap.Error(err))
				}
			}(logEntry)
		}
		wg.Wait()

		s.logger.Debug("Processed batch of invalid MSISDN logs",
			zap.Int("batchStart", i),
			zap.Int("batchEnd", end),
			zap.Int("batchSize", len(batch)))
	}
}

// batchCleanupInvalidMSISDNSubscriptions cleans up multiple invalid MSISDN subscriptions efficiently
func (s *SubscriptionService) batchCleanupInvalidMSISDNSubscriptions(cleanupTasks []struct {
	msisdn    string
	productId int
	requestID string
}) {
	if len(cleanupTasks) == 0 {
		return
	}

	// Process cleanup tasks concurrently with controlled concurrency
	maxConcurrency := 10
	semaphore := make(chan struct{}, maxConcurrency)

	var wg sync.WaitGroup
	for _, task := range cleanupTasks {
		wg.Add(1)
		go func(task struct {
			msisdn    string
			productId int
			requestID string
		}) {
			defer wg.Done()

			// Acquire semaphore to limit concurrency
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Process cleanup (productId is now only used for logging, cleanup is product-independent)
			s.handleInvalidMSISDNCleanup(task.msisdn, task.productId, task.requestID)
		}(task)
	}

	wg.Wait()
	s.logger.Info("Completed batch cleanup of invalid MSISDN subscriptions",
		zap.Int("totalTasks", len(cleanupTasks)))
}

// SendRenewalRequest sends a renewal request using the new renewal system
// DEPRECATED: This method is kept for backward compatibility but now delegates to the renewal service
func (s *SubscriptionService) SendRenewalRequest(msisdn string, product *domain.Product, entryChannel string) error {
	s.logger.Warn("SendRenewalRequest is deprecated, use renewalService.SendRenewalRequest instead",
		zap.String("msisdn", msisdn),
		zap.String("productId", product.ProductId))

	// Use the new renewal system
	ctx := context.Background()
	response, err := s.renewalService.SendRenewalRequest(ctx, msisdn, product, entryChannel)
	if err != nil {
		s.logger.Error("Error sending renewal request via renewal service",
			zap.String("msisdn", msisdn),
			zap.String("productId", product.ProductId),
			zap.Error(err))
		return fmt.Errorf("error sending renewal request via renewal service: %w", err)
	}

	// Log the renewal response
	s.logger.Info("Renewal request processed successfully via renewal service",
		zap.String("msisdn", msisdn),
		zap.String("productId", product.ProductId),
		zap.String("status", response.Status),
		zap.Bool("success", response.Success))

	return nil
}

// HandleAlreadyActiveSubscription handles the case when a subscription is already active
func (s *SubscriptionService) HandleAlreadyActiveSubscription(msisdn string, product *domain.Product, entryChannel string) error {
	s.logger.Info("Handling already active subscription",
		zap.String("msisdn", msisdn),
		zap.String("productId", product.ProductId))

	// Check if subscription exists in our database
	productId, err := strconv.Atoi(product.ProductId)
	if err != nil {
		s.logger.Error("Failed to convert ProductId", zap.Error(err))
		return fmt.Errorf("failed to convert ProductId: %w", err)
	}

	subscriptionExists, err := s.repo.CheckSubscriptionExists(msisdn, productId)
	if err != nil {
		s.logger.Error("Failed to check subscription existence", zap.Error(err))
		return fmt.Errorf("failed to check subscription existence: %w", err)
	}

	// If subscription doesn't exist in our database, insert it
	if !subscriptionExists {
		s.logger.Info("Subscription not found in database, inserting",
			zap.String("msisdn", msisdn),
			zap.String("productId", product.ProductId))

		// Create a dummy transaction ID for existing subscription
		txId := uuid.New().String()
		keyword := s.generateThreeLetterKeyword(product.Name)
		mtReq := domain.MTRequest{
			ProductID:          productId,
			PricepointID:       product.PricePointId,
			UserIdentifier:     msisdn,
			UserIdentifierType: "MSISDN",
			SubKeyword:         keyword,
			Context:            "Subscription",
			MCC:                "620",
			MNC:                "03",
			EntryChannel:       entryChannel,
			LargeAccount:       product.ShortCode,
			MoTransactionUUID:  txId,
			SendDate:           time.Now().Format(time.RFC3339),
		}

		partnerRoleId, err := strconv.Atoi(s.config.Application.TIMWE.PartnerRoleID)
		if err != nil {
			s.logger.Error("Failed to convert PartnerID", zap.Error(err))
			return fmt.Errorf("invalid PartnerID: %w", err)
		}

		subscriptionRequest := domain.MapMTRequestToSubscriptionRequest(mtReq, txId, partnerRoleId, "INTERNAL", "INTERNAL")

		if err := s.repo.CreateSubscription(&subscriptionRequest); err != nil {
			s.logger.Error("Error saving existing subscription", zap.Error(err))
			return fmt.Errorf("error saving existing subscription: %w", err)
		}

		s.logger.Info("Existing subscription saved to database",
			zap.String("msisdn", msisdn),
			zap.String("productId", product.ProductId))
	}

	// Check if renewal notification was sent this month
	renewalNotificationExists, err := s.repo.CheckRenewalNotificationExists(msisdn, productId)
	if err != nil {
		s.logger.Error("Failed to check renewal notification existence", zap.Error(err))
		return fmt.Errorf("failed to check renewal notification existence: %w", err)
	}

	// If no renewal notification was sent this month, send one
	if !renewalNotificationExists {
		s.logger.Info("No renewal notification found for current month, sending renewal request",
			zap.String("msisdn", msisdn),
			zap.String("productId", product.ProductId))

		if err := s.SendRenewalRequest(msisdn, product, entryChannel); err != nil {
			s.logger.Error("Error sending renewal request", zap.Error(err))
			return fmt.Errorf("error sending renewal request: %w", err)
		}

		s.logger.Info("Renewal request sent successfully",
			zap.String("msisdn", msisdn),
			zap.String("productId", product.ProductId))
	} else {
		s.logger.Info("Renewal notification already sent this month, skipping",
			zap.String("msisdn", msisdn),
			zap.String("productId", product.ProductId))
	}

	return nil
}

// HandleWaitingForChargingSubscription handles the case when a subscription is active and waiting for charging
func (s *SubscriptionService) HandleWaitingForChargingSubscription(msisdn string, product *domain.Product, entryChannel string, transactionIdStr string, partnerRoleId int, mtReq domain.MTRequest) error {
	ctx := context.Background()

	s.logger.Info("Handling waiting for charging subscription",
		zap.String("msisdn", msisdn),
		zap.String("productId", product.ProductId))

	// Check if subscription exists in our database with retry logic
	productId, err := strconv.Atoi(product.ProductId)
	if err != nil {
		s.logger.Error("Failed to convert ProductId", zap.Error(err))
		return fmt.Errorf("failed to convert ProductId: %w", err)
	}

	subscriptionExists, err := s.checkSubscriptionExistsWithRetry(msisdn, productId)
	if err != nil {
		return err
	}

	// If subscription doesn't exist in our database, insert it
	if !subscriptionExists {
		if err := s.createSubscriptionForChargingStatus(msisdn, product, entryChannel, partnerRoleId); err != nil {
			return err
		}
	}

	// Integrate with renewal system for intelligent charging status handling
	if s.renewalService != nil {
		if err := s.handleChargingStatusWithRenewalSystem(ctx, msisdn, product, entryChannel); err != nil {
			s.logger.Warn("Failed to handle charging status with renewal system", zap.Error(err))
			// Don't fail the main operation for this
		}
	}

	// Schedule periodic charging status monitoring
	go s.scheduleChargingStatusMonitoring(ctx, msisdn, product, transactionIdStr)

	s.logger.Info("Successfully handled waiting for charging subscription",
		zap.String("msisdn", msisdn),
		zap.String("productId", product.ProductId))

	return nil
}

// CheckChargingStatus checks the charging status for a subscription that is waiting for charging
func (s *SubscriptionService) CheckChargingStatus(msisdn string, product *domain.Product, transactionId string) error {
	s.logger.Info("Checking charging status for subscription",
		zap.String("msisdn", msisdn),
		zap.String("productId", product.ProductId),
		zap.String("transactionId", transactionId))

	// Use configurable MCC/MNC with fallbacks
	mcc := s.config.Application.TIMWE.MCC
	if mcc == "" {
		mcc = "620" // Default fallback
	}

	mnc := s.config.Application.TIMWE.MNC
	if mnc == "" {
		mnc = "03" // Default fallback
	}

	// Create a status check request
	statusReq := domain.GetStatusRequest{
		UserIdentifier:     msisdn,
		UserIdentifierType: "MSISDN",
		ProductId:          product.PricePointId,
		Mcc:                &mcc,                  // Use pointer to string
		Mnc:                &mnc,                  // Use pointer to string
		EntryChannel:       stringPtr("INTERNAL"), // Use pointer to string
		ClientIp:           stringPtr("INTERNAL"), // Use pointer to string
	}

	// Send status check request to TIMWE API
	realm := s.config.Application.TIMWE.Realm
	statusResponse, err := s.SendStatusCheck(statusReq, realm)
	if err != nil {
		s.logger.Error("Error checking charging status", zap.String("msisdn", msisdn), zap.Error(err))
		return fmt.Errorf("error checking charging status for msisdn %s: %w", msisdn, err)
	}

	// Log the status response
	s.logger.Info("Charging status response received",
		zap.String("msisdn", msisdn),
		zap.String("code", statusResponse.Code),
		zap.String("requestId", statusResponse.RequestID),
		zap.Any("responseData", statusResponse.ResponseData))

	s.logger.Info("Charging status check completed",
		zap.String("msisdn", msisdn),
		zap.String("productId", product.ProductId),
		zap.String("status", statusResponse.Code))

	return nil
}

func (s *SubscriptionService) buildTIMWEOptinPayload(reqData domain.MTRequest) (timweOptinPayload, error) {
	userIdentifier := strings.TrimSpace(reqData.UserIdentifier)
	if userIdentifier == "" {
		return timweOptinPayload{}, fmt.Errorf("invalid optin payload: userIdentifier is required")
	}
	if reqData.ProductID <= 0 {
		return timweOptinPayload{}, fmt.Errorf("invalid optin payload: productId must be greater than zero")
	}

	trackingID := defaultIfBlank(reqData.MoTransactionUUID, uuid.New().String())
	payload := timweOptinPayload{
		UserIdentifier:     userIdentifier,
		UserIdentifierType: defaultIfBlank(reqData.UserIdentifierType, timweDefaultMSISDNType),
		ProductID:          reqData.ProductID,
		MCC:                defaultIfBlank(reqData.MCC, s.getMCC()),
		MNC:                defaultIfBlank(reqData.MNC, s.getMNC()),
		EntryChannel:       defaultIfBlank(reqData.EntryChannel, timweDefaultEntryChannel),
		LargeAccount:       defaultIfBlank(reqData.LargeAccount, ""),
		SubKeyword:         defaultIfBlank(reqData.SubKeyword, ""),
		TrackingID:         trackingID,
		ClientIP:           timweDefaultClientIP,
		CampaignURL:        defaultIfBlank(reqData.CampaignUrl, timweDefaultClientIP),
	}
	return payload, nil
}

func (s *SubscriptionService) buildTIMWEOptinConfirmPayload(reqData domain.SubscriptionConfirmationRequest) (timweOptinConfirmPayload, error) {
	userIdentifier := strings.TrimSpace(reqData.UserIdentifier)
	if userIdentifier == "" {
		return timweOptinConfirmPayload{}, fmt.Errorf("invalid optin confirm payload: userIdentifier is required")
	}
	if reqData.ProductId <= 0 {
		return timweOptinConfirmPayload{}, fmt.Errorf("invalid optin confirm payload: productId must be greater than zero")
	}

	transactionAuthCode := strings.TrimSpace(reqData.TransactionAuthCode)
	if transactionAuthCode == "" {
		return timweOptinConfirmPayload{}, fmt.Errorf("invalid optin confirm payload: transactionAuthCode is required")
	}

	payload := timweOptinConfirmPayload{
		UserIdentifier:      userIdentifier,
		UserIdentifierType:  defaultIfBlank(reqData.UserIdentifierType, timweDefaultMSISDNType),
		ProductID:           reqData.ProductId,
		MCC:                 defaultFromPointer(reqData.Mcc, s.getMCC()),
		MNC:                 defaultFromPointer(reqData.Mnc, s.getMNC()),
		EntryChannel:        defaultFromPointer(reqData.EntryChannel, timweDefaultEntryChannel),
		ClientIP:            defaultFromPointer(reqData.ClientIp, timweDefaultClientIP),
		TransactionAuthCode: transactionAuthCode,
	}
	return payload, nil
}

func (s *SubscriptionService) buildTIMWEOptoutPayload(reqData domain.UnsubscriptionRequest) (timweOptoutPayload, error) {
	userIdentifier := strings.TrimSpace(reqData.UserIdentifier)
	if userIdentifier == "" {
		return timweOptoutPayload{}, fmt.Errorf("invalid optout payload: userIdentifier is required")
	}
	if reqData.ProductId <= 0 {
		return timweOptoutPayload{}, fmt.Errorf("invalid optout payload: productId must be greater than zero")
	}

	payload := timweOptoutPayload{
		UserIdentifier:        userIdentifier,
		UserIdentifierType:    defaultIfBlank(reqData.UserIdentifierType, timweDefaultMSISDNType),
		ProductID:             reqData.ProductId,
		MCC:                   defaultFromPointer(reqData.Mcc, s.getMCC()),
		MNC:                   defaultFromPointer(reqData.Mnc, s.getMNC()),
		EntryChannel:          defaultFromPointer(reqData.EntryChannel, timweDefaultEntryChannel),
		LargeAccount:          defaultFromPointer(reqData.LargeAccount, ""),
		SubKeyword:            defaultFromPointer(reqData.SubKeyword, ""),
		TrackingID:            defaultFromPointer(reqData.TrackingId, uuid.New().String()),
		ClientIP:              defaultFromPointer(reqData.ClientIp, timweDefaultClientIP),
		ControlKeyword:        defaultIfBlank(reqData.ControlKeyword, ""),
		ControlServiceKeyword: defaultIfBlank(reqData.ControlServiceKeyword, ""),
		SubID:                 reqData.SubId,
		CancelReason:          reqData.CancelReason,
		CancelSource:          reqData.CancelSource,
	}
	return payload, nil
}

func (s *SubscriptionService) buildTIMWEStatusPayload(reqData domain.GetStatusRequest) (timweStatusPayload, error) {
	userIdentifier := strings.TrimSpace(reqData.UserIdentifier)
	if userIdentifier == "" {
		return timweStatusPayload{}, fmt.Errorf("invalid status payload: userIdentifier is required")
	}
	if reqData.ProductId <= 0 {
		return timweStatusPayload{}, fmt.Errorf("invalid status payload: productId must be greater than zero")
	}

	payload := timweStatusPayload{
		UserIdentifier:        userIdentifier,
		UserIdentifierType:    defaultIfBlank(reqData.UserIdentifierType, timweDefaultMSISDNType),
		ProductID:             reqData.ProductId,
		MCC:                   defaultFromPointer(reqData.Mcc, s.getMCC()),
		MNC:                   defaultFromPointer(reqData.Mnc, s.getMNC()),
		EntryChannel:          defaultFromPointer(reqData.EntryChannel, timweDefaultEntryChannel),
		ClientIP:              defaultFromPointer(reqData.ClientIp, timweDefaultClientIP),
		ControlKeyword:        defaultIfBlank(reqData.ControlKeyword, ""),
		ControlServiceKeyword: defaultIfBlank(reqData.ControlServiceKeyword, ""),
		SubID:                 reqData.SubId,
	}
	return payload, nil
}

func defaultFromPointer(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return defaultIfBlank(*value, fallback)
}

func defaultIfBlank(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

// getMCC returns the configured MCC value with fallback
func (s *SubscriptionService) getMCC() string {
	if s.config.Application.TIMWE.MCC != "" {
		return s.config.Application.TIMWE.MCC
	}
	return "620" // Default fallback for Ghana
}

// getMNC returns the configured MNC value with fallback
func (s *SubscriptionService) getMNC() string {
	if s.config.Application.TIMWE.MNC != "" {
		return s.config.Application.TIMWE.MNC
	}
	return "03" // Default fallback for AirtelTigo
}

// checkSubscriptionExistsWithRetry checks subscription existence with retry logic
func (s *SubscriptionService) checkSubscriptionExistsWithRetry(msisdn string, productId int) (bool, error) {
	maxRetries := 3
	var subscriptionExists bool
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		subscriptionExists, err = s.repo.CheckSubscriptionExists(msisdn, productId)
		if err == nil {
			break
		}

		if attempt == maxRetries {
			s.logger.Error("Failed to check subscription existence after all retries",
				zap.String("msisdn", msisdn),
				zap.Error(err))
			return false, fmt.Errorf("failed to check subscription existence: %w", err)
		}

		// Exponential backoff
		delay := time.Duration(math.Pow(2, float64(attempt-1))) * 100 * time.Millisecond
		time.Sleep(delay)
		s.logger.Warn("Retrying subscription existence check",
			zap.Int("attempt", attempt),
			zap.Duration("delay", delay))
	}

	return subscriptionExists, nil
}

// createSubscriptionForChargingStatus creates a subscription for charging status handling
func (s *SubscriptionService) createSubscriptionForChargingStatus(msisdn string, product *domain.Product, entryChannel string, partnerRoleId int) error {
	s.logger.Info("Subscription not found in database, inserting",
		zap.String("msisdn", msisdn),
		zap.String("productId", product.ProductId))

	// Create a dummy transaction ID for existing subscription
	txId := uuid.New().String()
	keyword := s.generateThreeLetterKeyword(product.Name)

	// Use configurable MCC/MNC with fallbacks
	mcc := s.config.Application.TIMWE.MCC
	if mcc == "" {
		mcc = "620" // Default fallback
	}

	mnc := s.config.Application.TIMWE.MNC
	if mnc == "" {
		mnc = "03" // Default fallback
	}

	productId, err := strconv.Atoi(product.ProductId)
	if err != nil {
		s.logger.Error("Failed to convert ProductId in createSubscriptionForChargingStatus", zap.Error(err))
		return fmt.Errorf("failed to convert ProductId: %w", err)
	}

	mtReq := domain.MTRequest{
		ProductID:          productId,
		PricepointID:       product.PricePointId,
		UserIdentifier:     msisdn,
		UserIdentifierType: "MSISDN",
		SubKeyword:         keyword,
		Context:            "Subscription",
		MCC:                mcc,
		MNC:                mnc,
		EntryChannel:       entryChannel,
		LargeAccount:       product.ShortCode,
		MoTransactionUUID:  txId,
		SendDate:           time.Now().Format(time.RFC3339),
	}

	subscriptionRequest := domain.MapMTRequestToSubscriptionRequest(mtReq, txId, partnerRoleId, "INTERNAL", "INTERNAL")

	if err := s.repo.CreateSubscription(&subscriptionRequest); err != nil {
		s.logger.Error("Error saving existing subscription", zap.Error(err))
		return fmt.Errorf("error saving existing subscription: %w", err)
	}

	s.logger.Info("Existing subscription saved to database",
		zap.String("msisdn", msisdn),
		zap.String("productId", product.ProductId))

	return nil
}

// handleChargingStatusWithRenewalSystem integrates charging status handling with the renewal system
func (s *SubscriptionService) handleChargingStatusWithRenewalSystem(ctx context.Context, msisdn string, product *domain.Product, entryChannel string) error {
	if s.renewalService == nil {
		return fmt.Errorf("renewal service not available")
	}

	// Evaluate if this subscription needs renewal attention
	churnAction := s.renewalService.EvaluateChurnPolicy(ctx, msisdn, product.ProductId)

	switch churnAction {
	case domain.ActionGracePeriod:
		s.logger.Info("Subscription in grace period, scheduling renewal",
			zap.String("msisdn", msisdn),
			zap.String("productId", product.ProductId))

		// Schedule renewal via renewal service
		if err := s.scheduleRenewalForChargingStatus(msisdn, product, entryChannel); err != nil {
			s.logger.Warn("Failed to schedule renewal for charging status", zap.Error(err))
			return err
		}

	case domain.ActionChurn:
		s.logger.Warn("Subscription marked for churn due to charging issues",
			zap.String("msisdn", msisdn),
			zap.String("productId", product.ProductId))

		// Process churn via renewal service
		if err := s.renewalService.ChurnSubscription(ctx, msisdn, product.ProductId, "charging_failure"); err != nil {
			s.logger.Error("Failed to process churn for charging status", zap.Error(err))
			return err
		}

	case domain.ActionNoAction:
		s.logger.Info("Subscription in normal state, no renewal action needed",
			zap.String("msisdn", msisdn),
			zap.String("productId", product.ProductId))

	default:
		s.logger.Info("Unknown churn action, treating as normal",
			zap.String("msisdn", msisdn),
			zap.String("productId", product.ProductId),
			zap.String("action", string(churnAction)))
	}

	return nil
}

// scheduleRenewalForChargingStatus schedules a renewal for subscriptions with charging status issues
func (s *SubscriptionService) scheduleRenewalForChargingStatus(msisdn string, product *domain.Product, entryChannel string) error {
	// Since we're in SubscriptionService, we'll use the renewal service to handle this
	// The renewal service will create the appropriate renewal cycle
	if s.renewalService != nil {
		ctx := context.Background()

		// Use the renewal service to send a renewal request
		response, err := s.renewalService.SendRenewalRequest(ctx, msisdn, product, entryChannel)
		if err != nil {
			s.logger.Error("Failed to schedule renewal via renewal service", zap.Error(err))
			return fmt.Errorf("failed to schedule renewal via renewal service: %w", err)
		}

		s.logger.Info("Renewal scheduled for charging status subscription via renewal service",
			zap.String("msisdn", msisdn),
			zap.String("productId", product.ProductId),
			zap.String("status", response.Status))
	} else {
		s.logger.Warn("Renewal service not available, skipping renewal scheduling",
			zap.String("msisdn", msisdn),
			zap.String("productId", product.ProductId))
	}

	return nil
}

// scheduleChargingStatusMonitoring schedules periodic charging status monitoring
func (s *SubscriptionService) scheduleChargingStatusMonitoring(ctx context.Context, msisdn string, product *domain.Product, transactionIdStr string) {
	// Wait before first check
	time.Sleep(5 * time.Minute)

	// Check charging status periodically
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	checkCount := 0
	maxChecks := 8 // Monitor for 4 hours (8 * 30 minutes)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Charging status monitoring stopped due to context cancellation",
				zap.String("msisdn", msisdn),
				zap.String("productId", product.ProductId))
			return
		case <-ticker.C:
			checkCount++

			s.logger.Info("Performing periodic charging status check",
				zap.String("msisdn", msisdn),
				zap.String("productId", product.ProductId),
				zap.Int("checkCount", checkCount),
				zap.Int("maxChecks", maxChecks))

			if err := s.CheckChargingStatus(msisdn, product, transactionIdStr); err != nil {
				s.logger.Error("Periodic charging status check failed",
					zap.String("msisdn", msisdn),
					zap.Error(err))
			}

			// Stop monitoring after max checks
			if checkCount >= maxChecks {
				s.logger.Info("Charging status monitoring completed",
					zap.String("msisdn", msisdn),
					zap.String("productId", product.ProductId),
					zap.Int("totalChecks", checkCount))
				return
			}
		}
	}
}

// SendStatusCheck sends a status check request to the TIMWE API
func (s *SubscriptionService) SendStatusCheck(reqData domain.GetStatusRequest, realm string) (*domain.MTResponse, error) {
	release := s.acquireBulkhead()
	defer release()

	done, err := s.circuitBreaker.Allow()
	if err != nil {
		s.logger.Error("Circuit breaker open for SendStatusCheck", zap.Error(err))
		return nil, err
	}
	result, callErr := s.sendStatusCheckWithRetry(reqData, realm)
	success := callErr == nil || s.isNonBreakerError(callErr)
	done(success)
	if callErr != nil {
		if success {
			return nil, callErr
		}
		s.logger.Error("Circuit breaker classified failure in SendStatusCheck", zap.Error(callErr))
		return nil, callErr
	}

	// Check if we need to retry with SMS entry channel for OPTIN_CONFIG_NOT_FOUND
	if s.shouldRetryWithSMS(result) {
		entryChannel := "INTERNAL"
		if reqData.EntryChannel != nil {
			entryChannel = *reqData.EntryChannel
		}

		s.logger.Info("OPTIN_CONFIG_NOT_FOUND detected in SendStatusCheck, retrying with SMS entry channel",
			zap.String("msisdn", reqData.UserIdentifier),
			zap.String("originalChannel", entryChannel))

		// Retry with SMS entry channel
		statusReqCopy := reqData
		statusReqCopy.EntryChannel = stringPtr("SMS")
		result, callErr = s.sendStatusCheckWithRetry(statusReqCopy, realm)
		if callErr != nil {
			s.logger.Error("Error sending status check retry with SMS", zap.String("msisdn", reqData.UserIdentifier), zap.Error(callErr))
			return nil, callErr
		}

		s.logger.Info("SendStatusCheck retry with SMS completed",
			zap.String("msisdn", reqData.UserIdentifier),
			zap.String("retryChannel", "SMS"),
			zap.String("responseCode", result.Code))
	}

	return result, nil
}

// sendStatusCheckWithRetry handles the actual status check request with retry logic for INTERNAL_ERROR
func (s *SubscriptionService) sendStatusCheckWithRetry(reqData domain.GetStatusRequest, realm string) (*domain.MTResponse, error) {
	providerCfg, err := s.providerConfigOrLegacy(context.Background(), ChannelOperationStatus, reqData.TenantRoute)
	if err != nil {
		return nil, err
	}
	reqData.TenantRoute = canonicalTenantRoute(reqData.TenantRoute, providerCfg)
	authKey, err := providerCfg.AuthKey()
	if err != nil {
		s.logger.Error("failed to resolve auth key", zap.Error(err))
		return nil, fmt.Errorf("failed to resolve auth key: %w", err)
	}
	partnerRoleID, err := providerCfg.PartnerRoleInt()
	if err != nil {
		s.logger.Error("Failed to convert PartnerRoleID", zap.Error(err))
		return nil, err
	}

	url := fmt.Sprintf("%s/subscription/status/%d", providerCfg.BaseURL, partnerRoleID)
	payload, err := s.buildTIMWEStatusPayload(reqData)
	if err != nil {
		s.logger.Error("Failed to normalize status payload", zap.Error(err))
		return nil, err
	}

	requestBody, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("Failed to marshal request data", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal request data: %v", err)
	}

	// Log the status check request details for debugging
	s.logger.Info("Sending status check request",
		zap.String("url", url),
		zap.String("method", "POST"),
		zap.String("apikey", "[REDACTED]"),
		zap.String("requestBody", string(requestBody)))

	maxRetries := 3
	baseDelay := 200 * time.Millisecond

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req := fasthttp.AcquireRequest()
		res := fasthttp.AcquireResponse()
		externalTxID := uuid.New().String()

		// Set up request
		req.SetRequestURI(url)
		req.Header.SetMethod("POST")
		req.Header.Set("apikey", providerCfg.APIKey)
		req.Header.Set("authentication", authKey)
		req.Header.Set("external-tx-id", externalTxID)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "*/*")
		req.SetBody(requestBody)

		// Send request
		if err = s.client.Do(req, res); err != nil {
			s.logger.Warn("Failed to send status check request",
				zap.Int("attempt", attempt),
				zap.String("msisdn", reqData.UserIdentifier),
				zap.Error(err))

			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(res)

			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to check status after %d attempts: %v", maxRetries, err)
			}

			// Exponential backoff for network errors
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			time.Sleep(delay)
			continue
		}

		// Check HTTP status code
		if res.StatusCode() != fasthttp.StatusOK {
			s.logger.Error("Status check request failed with non-200 status",
				zap.Int("attempt", attempt),
				zap.Int("statusCode", res.StatusCode()),
				zap.String("responseBody", string(res.Body())),
				zap.String("msisdn", reqData.UserIdentifier),
				zap.String("url", url))

			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(res)

			if attempt == maxRetries {
				return nil, fmt.Errorf("status check request failed with status code: %d", res.StatusCode())
			}

			// Exponential backoff for HTTP errors
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			time.Sleep(delay)
			continue
		}

		// Parse response
		var mtResponse domain.MTResponse
		if err := json.Unmarshal(res.Body(), &mtResponse); err != nil {
			s.logger.Error("Failed to parse status check response",
				zap.Int("attempt", attempt),
				zap.String("msisdn", reqData.UserIdentifier),
				zap.Error(err))

			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(res)

			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to parse status check response: %v", err)
			}

			// Exponential backoff for parsing errors
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			time.Sleep(delay)
			continue
		}

		// Log the status check response for debugging
		s.logger.Info("Status check API response received",
			zap.Int("attempt", attempt),
			zap.String("msisdn", reqData.UserIdentifier),
			zap.String("code", mtResponse.Code),
			zap.Bool("inError", mtResponse.InError),
			zap.String("requestId", mtResponse.RequestID),
			zap.Any("responseData", mtResponse.ResponseData))

		// Check for INTERNAL_ERROR and retry if needed
		if mtResponse.Code == ResponseCodeInternalError {
			s.logger.Warn("Status check request failed with internal error, retrying",
				zap.Int("attempt", attempt),
				zap.String("msisdn", reqData.UserIdentifier),
				zap.String("requestId", mtResponse.RequestID))

			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(res)

			if attempt == maxRetries {
				s.logger.Error("Status check request failed with internal error after all retries",
					zap.String("msisdn", reqData.UserIdentifier),
					zap.String("requestId", mtResponse.RequestID))
				return nil, fmt.Errorf("status check request failed with internal error after %d attempts: requestId=%s", maxRetries, mtResponse.RequestID)
			}

			// Exponential backoff for INTERNAL_ERROR
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			s.logger.Info("Retrying status check request after INTERNAL_ERROR",
				zap.Int("attempt", attempt+1),
				zap.Duration("delay", delay),
				zap.String("msisdn", reqData.UserIdentifier))
			time.Sleep(delay)
			continue
		}

		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(res)
		return &mtResponse, nil
	}

	return nil, fmt.Errorf("unexpected end of retry loop")
}

func (s *SubscriptionService) SendOptout(reqData domain.UnsubscriptionRequest, realm string) (*domain.MTResponse, error) {
	release := s.acquireBulkhead()
	defer release()

	// Guard with circuit breaker (TwoStep)
	done, err := s.circuitBreaker.Allow()
	if err != nil {
		s.logger.Error("Circuit breaker open for SendOptout", zap.Error(err))
		return nil, err
	}
	resp, callErr := s.sendOptoutWithRetry(reqData, realm)
	success := callErr == nil || s.isNonBreakerError(callErr)
	done(success)
	if callErr != nil {
		if success {
			return nil, callErr
		}
		s.logger.Error("Circuit breaker classified failure in SendOptout", zap.Error(callErr))
		return nil, callErr
	}

	// Check if we need to retry with SMS entry channel for OPTIN_CONFIG_NOT_FOUND
	if s.shouldRetryWithSMS(resp) {
		entryChannel := "INTERNAL"
		if reqData.EntryChannel != nil {
			entryChannel = *reqData.EntryChannel
		}

		s.logger.Info("OPTIN_CONFIG_NOT_FOUND detected in SendOptout, retrying with SMS entry channel",
			zap.String("msisdn", reqData.UserIdentifier),
			zap.String("originalChannel", entryChannel))

		// Retry with SMS entry channel
		optoutReqCopy := reqData
		optoutReqCopy.EntryChannel = stringPtr("SMS")
		resp, callErr = s.sendOptoutWithRetry(optoutReqCopy, realm)
		if callErr != nil {
			s.logger.Error("Error sending optout retry with SMS", zap.String("msisdn", reqData.UserIdentifier), zap.Error(callErr))
			return nil, callErr
		}

		s.logger.Info("SendOptout retry with SMS completed",
			zap.String("msisdn", reqData.UserIdentifier),
			zap.String("retryChannel", "SMS"),
			zap.String("responseCode", resp.Code))
	}

	return resp, nil
}

// sendOptoutWithRetry handles the actual opt-out request with retry logic similar to status/MT flows
func (s *SubscriptionService) sendOptoutWithRetry(reqData domain.UnsubscriptionRequest, realm string) (*domain.MTResponse, error) {
	providerCfg, err := s.providerConfigOrLegacy(context.Background(), ChannelOperationOptout, reqData.TenantRoute)
	if err != nil {
		return nil, err
	}
	reqData.TenantRoute = canonicalTenantRoute(reqData.TenantRoute, providerCfg)
	authKey, err := providerCfg.AuthKey()
	if err != nil {
		s.logger.Error("failed to resolve auth key", zap.Error(err))
		return nil, fmt.Errorf("failed to resolve auth key: %w", err)
	}
	partnerRoleID, err := providerCfg.PartnerRoleInt()
	if err != nil {
		s.logger.Error("Failed to convert PartnerRoleID", zap.Error(err))
		return nil, err
	}

	url := fmt.Sprintf("%s/subscription/optout/%d", providerCfg.BaseURL, partnerRoleID)
	payload, err := s.buildTIMWEOptoutPayload(reqData)
	if err != nil {
		s.logger.Error("Failed to normalize optout payload", zap.Error(err))
		return nil, err
	}

	requestBody, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("Failed to marshal unsubscription request", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal unsubscription request: %w", err)
	}

	maxRetries := 3
	baseDelay := 200 * time.Millisecond
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req := fasthttp.AcquireRequest()
		res := fasthttp.AcquireResponse()
		externalTxID := uuid.New().String()

		// Prepare request
		req.SetRequestURI(url)
		req.Header.SetMethod("POST")
		req.Header.Set("apikey", providerCfg.APIKey)
		req.Header.Set("authentication", authKey)
		req.Header.Set("external-tx-id", externalTxID)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "*/*")
		req.SetBody(requestBody)

		// Execute
		if err = s.client.Do(req, res); err != nil {
			s.logger.Warn("Failed to send optout request",
				zap.Int("attempt", attempt),
				zap.String("msisdn", reqData.UserIdentifier),
				zap.Error(err))
			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(res)
			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to send optout after %d attempts: %v", maxRetries, err)
			}
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			time.Sleep(delay)
			continue
		}

		// HTTP status check
		if res.StatusCode() != fasthttp.StatusOK {
			s.logger.Error("Optout request failed with non-200 status",
				zap.Int("attempt", attempt),
				zap.Int("statusCode", res.StatusCode()),
				zap.String("responseBody", string(res.Body())),
				zap.String("msisdn", reqData.UserIdentifier),
				zap.String("url", url))
			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(res)
			if attempt == maxRetries {
				return nil, fmt.Errorf("optout request failed with status code: %d", res.StatusCode())
			}
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			time.Sleep(delay)
			continue
		}

		// Parse response
		var mtResponse domain.MTResponse
		if err := json.Unmarshal(res.Body(), &mtResponse); err != nil {
			s.logger.Error("Failed to parse optout response",
				zap.Int("attempt", attempt),
				zap.String("msisdn", reqData.UserIdentifier),
				zap.Error(err))
			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(res)
			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to parse optout response: %v", err)
			}
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			time.Sleep(delay)
			continue
		}

		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(res)

		// Check if we need to retry with SMS entry channel for OPTIN_CONFIG_NOT_FOUND
		if s.shouldRetryWithSMS(&mtResponse) {
			entryChannel := "INTERNAL"
			if reqData.EntryChannel != nil {
				entryChannel = *reqData.EntryChannel
			}

			s.logger.Info("OPTIN_CONFIG_NOT_FOUND detected in SendOptout, retrying with SMS entry channel",
				zap.String("msisdn", reqData.UserIdentifier),
				zap.String("originalChannel", entryChannel))

			// Retry with SMS entry channel
			optoutReqCopy := reqData
			optoutReqCopy.EntryChannel = stringPtr("SMS")
			result, callErr := s.sendOptoutWithRetry(optoutReqCopy, realm)
			if callErr != nil {
				s.logger.Error("Error sending optout retry with SMS", zap.String("msisdn", reqData.UserIdentifier), zap.Error(callErr))
				return nil, callErr
			}

			s.logger.Info("SendOptout retry with SMS completed",
				zap.String("msisdn", reqData.UserIdentifier),
				zap.String("retryChannel", "SMS"),
				zap.String("responseCode", result.Code))
		}

		return &mtResponse, nil
	}

	return nil, fmt.Errorf("unexpected end of retry loop")
}

// validateOptoutResponse treats any SUCCESS code as success for opt-out flows, even if responseData is empty
func (s *SubscriptionService) validateOptoutResponse(response *domain.MTResponse) error {
	if response.Code == ResponseCodeSuccess {
		if response.ResponseData == nil {
			response.ResponseData = map[string]interface{}{}
		}
		return nil
	}
	return s.validateMTResponse(response, domain.MTRequest{})
}

// SendOptinConfirm sends confirmation for double opt-in flow to TIMWE API
func (s *SubscriptionService) SendOptinConfirm(reqData domain.SubscriptionConfirmationRequest, realm string) (*domain.MTResponse, error) {
	release := s.acquireBulkhead()
	defer release()

	done, err := s.circuitBreaker.Allow()
	if err != nil {
		s.logger.Error("Circuit breaker open for SendOptinConfirm", zap.Error(err))
		return nil, err
	}
	resp, callErr := s.sendOptinConfirmWithRetry(reqData, realm)
	success := callErr == nil || s.isNonBreakerError(callErr)
	done(success)
	if callErr != nil {
		if success {
			return nil, callErr
		}
		s.logger.Error("Circuit breaker classified failure in SendOptinConfirm", zap.Error(callErr))
		return nil, callErr
	}

	// Check if we need to retry with SMS entry channel for OPTIN_CONFIG_NOT_FOUND
	if s.shouldRetryWithSMS(resp) {
		entryChannel := "INTERNAL"
		if reqData.EntryChannel != nil {
			entryChannel = *reqData.EntryChannel
		}

		s.logger.Info("OPTIN_CONFIG_NOT_FOUND detected in SendOptinConfirm, retrying with SMS entry channel",
			zap.String("userIdentifier", reqData.UserIdentifier),
			zap.String("originalChannel", entryChannel))

		// Retry with SMS entry channel
		confirmReqCopy := reqData
		confirmReqCopy.EntryChannel = stringPtr("SMS")
		result, callErr := s.sendOptinConfirmWithRetry(confirmReqCopy, realm)
		if callErr != nil {
			s.logger.Error("Error sending optin confirm retry with SMS", zap.String("userIdentifier", reqData.UserIdentifier), zap.Error(callErr))
			return nil, callErr
		}

		s.logger.Info("SendOptinConfirm retry with SMS completed",
			zap.String("userIdentifier", reqData.UserIdentifier),
			zap.String("retryChannel", "SMS"),
			zap.String("responseCode", result.Code))
	}

	return resp, nil
}

func (s *SubscriptionService) sendOptinConfirmWithRetry(reqData domain.SubscriptionConfirmationRequest, realm string) (*domain.MTResponse, error) {
	providerCfg, err := s.providerConfigOrLegacy(context.Background(), ChannelOperationConfirm, reqData.TenantRoute)
	if err != nil {
		return nil, err
	}
	reqData.TenantRoute = canonicalTenantRoute(reqData.TenantRoute, providerCfg)
	authKey, err := providerCfg.AuthKey()
	if err != nil {
		s.logger.Error("failed to resolve auth key", zap.Error(err))
		return nil, fmt.Errorf("failed to resolve auth key: %w", err)
	}
	partnerRoleID, err := providerCfg.PartnerRoleInt()
	if err != nil {
		s.logger.Error("Failed to convert PartnerRoleID", zap.Error(err))
		return nil, err
	}

	url := fmt.Sprintf("%s/subscription/optin/confirm/%d", providerCfg.BaseURL, partnerRoleID)
	payload, err := s.buildTIMWEOptinConfirmPayload(reqData)
	if err != nil {
		s.logger.Error("Failed to normalize optin confirm payload", zap.Error(err))
		return nil, err
	}

	requestBody, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("Failed to marshal confirmation request", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal confirmation request: %v", err)
	}

	maxRetries := 3
	baseDelay := 200 * time.Millisecond
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req := fasthttp.AcquireRequest()
		res := fasthttp.AcquireResponse()
		externalTxID := uuid.New().String()

		req.SetRequestURI(url)
		req.Header.SetMethod("POST")
		req.Header.Set("apikey", providerCfg.APIKey)
		req.Header.Set("authentication", authKey)
		req.Header.Set("external-tx-id", externalTxID)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "*/*")
		req.SetBody(requestBody)

		if err = s.client.Do(req, res); err != nil {
			s.logger.Warn("Failed to send optin confirm request",
				zap.Int("attempt", attempt),
				zap.String("userIdentifier", reqData.UserIdentifier),
				zap.Error(err))
			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(res)
			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to send optin confirm after %d attempts: %v", maxRetries, err)
			}
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			time.Sleep(delay)
			continue
		}

		if res.StatusCode() != fasthttp.StatusOK {
			s.logger.Error("Optin confirm request failed with non-200 status",
				zap.Int("attempt", attempt),
				zap.Int("statusCode", res.StatusCode()),
				zap.String("responseBody", string(res.Body())),
				zap.String("userIdentifier", reqData.UserIdentifier),
				zap.String("url", url))
			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(res)
			if attempt == maxRetries {
				return nil, fmt.Errorf("optin confirm request failed with status code: %d", res.StatusCode())
			}
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			time.Sleep(delay)
			continue
		}

		var mtResponse domain.MTResponse
		if err := json.Unmarshal(res.Body(), &mtResponse); err != nil {
			s.logger.Error("Failed to parse optin confirm response",
				zap.Int("attempt", attempt),
				zap.String("userIdentifier", reqData.UserIdentifier),
				zap.Error(err))
			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(res)
			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to parse optin confirm response: %v", err)
			}
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			time.Sleep(delay)
			continue
		}

		s.logger.Info("Optin confirm API response received",
			zap.Int("attempt", attempt),
			zap.String("userIdentifier", reqData.UserIdentifier),
			zap.String("code", mtResponse.Code),
			zap.Bool("inError", mtResponse.InError),
			zap.String("requestId", mtResponse.RequestID),
			zap.Any("responseData", mtResponse.ResponseData))

		// Check for INTERNAL_ERROR and retry if needed
		if mtResponse.Code == ResponseCodeInternalError {
			s.logger.Warn("Optin confirm failed with INTERNAL_ERROR, retrying",
				zap.Int("attempt", attempt),
				zap.String("userIdentifier", reqData.UserIdentifier),
				zap.String("requestId", mtResponse.RequestID))

			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(res)

			if attempt == maxRetries {
				s.logger.Error("Optin confirm failed with INTERNAL_ERROR after all retries",
					zap.String("userIdentifier", reqData.UserIdentifier),
					zap.String("requestId", mtResponse.RequestID))
				return nil, fmt.Errorf("optin confirm failed with internal error after %d attempts: requestId=%s", maxRetries, mtResponse.RequestID)
			}

			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			s.logger.Info("Retrying optin confirm after INTERNAL_ERROR",
				zap.Int("attempt", attempt+1),
				zap.Duration("delay", delay),
				zap.String("userIdentifier", reqData.UserIdentifier))
			time.Sleep(delay)
			continue
		}

		if err := s.validateMTResponse(&mtResponse, domain.MTRequest{UserIdentifier: reqData.UserIdentifier}); err != nil {
			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(res)
			return nil, err
		}

		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(res)
		return &mtResponse, nil
	}

	return nil, fmt.Errorf("unexpected end of retry loop")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// generateThreeLetterKeyword returns a three-letter uppercase keyword derived from the product name.
// Example: "Road Safety" -> "RST" (first letters of first three alphanumeric words),
//
//	"Music" -> "MUS" (first three letters),
//	"Go+ Play" -> "GOP".
func (s *SubscriptionService) generateThreeLetterKeyword(name string) string {
	// Normalize spaces and split
	fields := strings.FieldsFunc(name, func(r rune) bool {
		return unicode.IsSpace(r) || r == '-' || r == '_' || r == '/'
	})
	var letters []rune
	// Take first letter of up to three alphanumeric words
	for _, f := range fields {
		for _, r := range f {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				letters = append(letters, unicode.ToUpper(r))
				break
			}
		}
		if len(letters) >= 3 {
			break
		}
	}
	// If fewer than three initials, pad by taking next letters from first word
	if len(letters) < 3 && len(fields) > 0 {
		first := []rune(strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return unicode.ToUpper(r)
			}
			return -1
		}, fields[0]))
		for i := 0; i < len(first) && len(letters) < 3; i++ {
			// Skip if already used as initial
			if len(letters) > 0 && first[i] == letters[0] {
				continue
			}
			letters = append(letters, first[i])
		}
	}
	// Ensure length 3 by padding with X
	for len(letters) < 3 {
		letters = append(letters, 'X')
	}
	return string(letters[:3])
}

// GetRepository returns the repository interface for external access
func (s *SubscriptionService) GetRepository() repository.SubscriptionRepositoryInterface {
	return s.repo
}

// handleBlacklistedUser adds a user to the blacklist and removes their subscriptions
func (s *SubscriptionService) handleBlacklistedUser(msisdn string, response *domain.MTResponse) error {
	s.logger.Info("Processing BLACKLISTED user",
		zap.String("msisdn", msisdn),
		zap.String("requestId", response.RequestID))

	// Add user to blacklist in userbase
	if err := s.addUserToBlacklist(msisdn); err != nil {
		s.logger.Error("Failed to add user to blacklist",
			zap.String("msisdn", msisdn),
			zap.Error(err))
		return fmt.Errorf("failed to add user to blacklist: %w", err)
	}

	// Remove user's subscriptions
	if err := s.removeUserSubscriptions(msisdn); err != nil {
		s.logger.Error("Failed to remove user subscriptions",
			zap.String("msisdn", msisdn),
			zap.Error(err))
		return fmt.Errorf("failed to remove user subscriptions: %w", err)
	}

	s.logger.Info("Successfully processed BLACKLISTED user",
		zap.String("msisdn", msisdn),
		zap.String("requestId", response.RequestID))

	return nil
}

// addUserToBlacklist adds a user to the blacklist in the userbase
func (s *SubscriptionService) addUserToBlacklist(msisdn string) error {
	// Create blacklist user record
	blacklistUser := &domain.UserBase{
		Msisdn: msisdn,
		Type:   "BLACKLISTED",
	}

	// Insert or update the user in userbase
	if err := s.UserBaseRepository.InsertUserRecords(context.Background(), []*domain.UserBase{blacklistUser}); err != nil {
		return fmt.Errorf("failed to insert blacklisted user: %w", err)
	}

	s.logger.Info("Successfully added user to blacklist",
		zap.String("msisdn", msisdn))

	return nil
}

// removeUserSubscriptions removes all subscriptions for a blacklisted user
func (s *SubscriptionService) removeUserSubscriptions(msisdn string) error {
	// Use the existing DeleteSubscriptionRecord method to remove all subscriptions
	if err := s.repo.DeleteSubscriptionRecord(msisdn); err != nil {
		return fmt.Errorf("failed to delete subscription records: %w", err)
	}

	s.logger.Info("Successfully removed all subscriptions for blacklisted user",
		zap.String("msisdn", msisdn))

	return nil
}

// Stop gracefully shuts down the subscription service
func (s *SubscriptionService) Stop() {
	s.logger.Info("Stopping subscription service...")

	// Close all idle connections
	s.client.CloseIdleConnections()

	// Stop the cleanup goroutine
	if s.cleanupTicker != nil {
		s.cleanupTicker.Stop()
	}

	s.logger.Info("Subscription service stopped")
}

// stringPtr returns a pointer to the given string value
func stringPtr(s string) *string {
	return &s
}

// handleBlacklistedUserEnhanced handles the enhanced processing of blacklisted users asynchronously
func (s *SubscriptionService) handleBlacklistedUserEnhanced(msisdn string, productId int, requestID string, partnerId int, response *domain.MTResponse) {
	startTime := time.Now()
	success := false
	defer func() {
		// Track metrics
		duration := time.Since(startTime)
		if success {
			s.logger.Info("Enhanced BLACKLISTED user processing completed successfully",
				zap.String("msisdn", msisdn),
				zap.Int("productId", productId),
				zap.String("requestId", requestID),
				zap.Duration("duration", duration))
		} else {
			s.logger.Error("Enhanced BLACKLISTED user processing failed",
				zap.String("msisdn", msisdn),
				zap.Int("productId", productId),
				zap.String("requestId", requestID),
				zap.Duration("duration", duration))
		}
	}()

	// Step 1: Add user to blacklist in userbase with retry logic
	if err := s.addUserToBlacklistWithRetry(msisdn, productId, requestID, partnerId, response); err != nil {
		s.logger.Error("Failed to add user to blacklist with retry",
			zap.String("msisdn", msisdn),
			zap.Error(err))
		return
	}

	// Step 2: Check if user has subscriptions and remove them with retry logic
	if err := s.removeUserSubscriptionsWithRetry(msisdn); err != nil {
		s.logger.Error("Failed to remove user subscriptions with retry",
			zap.String("msisdn", msisdn),
			zap.Error(err))
		return
	}

	// Step 3: Create audit log entry
	if err := s.createBlacklistedUserAuditLog(msisdn, productId, requestID, partnerId, response); err != nil {
		s.logger.Warn("Failed to create audit log entry",
			zap.String("msisdn", msisdn),
			zap.Error(err))
		// Don't fail the entire operation for audit log failure
	}

	success = true
	s.logger.Info("Successfully processed enhanced BLACKLISTED user",
		zap.String("msisdn", msisdn),
		zap.String("requestId", requestID))
}

// addUserToBlacklistWithRetry adds a user to the blacklist with retry logic
func (s *SubscriptionService) addUserToBlacklistWithRetry(msisdn string, productId int, requestID string, partnerId int, response *domain.MTResponse) error {
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := s.addUserToBlacklistEnhanced(msisdn, productId, requestID, partnerId, response); err == nil {
			s.logger.Info("Successfully added user to blacklist",
				zap.String("msisdn", msisdn),
				zap.Int("attempt", attempt))
			return nil
		} else {
			// Log retry attempt
			s.logger.Warn("Failed to add user to blacklist, retrying",
				zap.String("msisdn", msisdn),
				zap.Int("attempt", attempt),
				zap.Int("maxRetries", maxRetries),
				zap.Error(err))

			// Wait before retry with exponential backoff
			if attempt < maxRetries {
				backoffDuration := time.Duration(attempt*attempt) * 100 * time.Millisecond
				time.Sleep(backoffDuration)
			}
		}
	}

	return fmt.Errorf("failed to add user to blacklist after %d retries", maxRetries)
}

// addUserToBlacklistEnhanced adds a user to the blacklist with enhanced logging and metadata
func (s *SubscriptionService) addUserToBlacklistEnhanced(msisdn string, productId int, requestID string, partnerId int, response *domain.MTResponse) error {
	// Create enhanced blacklist user record
	blacklistUser := &domain.UserBase{
		Msisdn: msisdn,
		Type:   "BLACKLISTED",
	}

	// Insert or update the user in userbase
	if err := s.UserBaseRepository.InsertUserRecords(context.Background(), []*domain.UserBase{blacklistUser}); err != nil {
		return fmt.Errorf("failed to insert blacklisted user: %w", err)
	}

	s.logger.Info("Successfully added user to blacklist (enhanced)",
		zap.String("msisdn", msisdn),
		zap.Int("productId", productId),
		zap.String("requestId", requestID),
		zap.Int("partnerId", partnerId))

	return nil
}

// removeUserSubscriptionsWithRetry removes user subscriptions with retry logic
func (s *SubscriptionService) removeUserSubscriptionsWithRetry(msisdn string) error {
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := s.repo.DeleteSubscriptionRecord(msisdn); err == nil {
			s.logger.Info("Successfully removed user subscriptions",
				zap.String("msisdn", msisdn),
				zap.Int("attempt", attempt))
			return nil
		} else {
			// Log retry attempt
			s.logger.Warn("Failed to remove user subscriptions, retrying",
				zap.String("msisdn", msisdn),
				zap.Int("attempt", attempt),
				zap.Int("maxRetries", maxRetries),
				zap.Error(err))

			// Wait before retry with exponential backoff
			if attempt < maxRetries {
				backoffDuration := time.Duration(attempt*attempt) * 100 * time.Millisecond
				time.Sleep(backoffDuration)
			}
		}
	}

	return fmt.Errorf("failed to remove user subscriptions after %d retries", maxRetries)
}

// createBlacklistedUserAuditLog creates an audit log entry for blacklisted user operations
func (s *SubscriptionService) createBlacklistedUserAuditLog(msisdn string, productId int, requestID string, partnerId int, response *domain.MTResponse) error {
	// Create audit log entry
	auditLog := &domain.BlacklistedUserAudit{
		Msisdn:        msisdn,
		Action:        "USER_BLACKLISTED",
		PreviousState: "ACTIVE",
		NewState:      "BLACKLISTED",
		UserID:        "SYSTEM",
		IPAddress:     "N/A",
		UserAgent:     "N/A",
		Timestamp:     time.Now(),
		Reason:        "MT Response indicated BLACKLISTED status",
		Metadata:      fmt.Sprintf("productId:%d,partnerId:%d,requestId:%s", productId, partnerId, requestID),
	}

	// For now, just log the audit entry since we don't have the repository method yet
	s.logger.Info("Blacklisted user audit log entry created",
		zap.String("msisdn", msisdn),
		zap.String("action", auditLog.Action),
		zap.String("reason", auditLog.Reason),
		zap.String("metadata", auditLog.Metadata))

	return nil
}

// BatchHandleBlacklistedUsers processes multiple BLACKLISTED responses efficiently
func (s *SubscriptionService) BatchHandleBlacklistedUsers(responses []*domain.MTResponse, requests []domain.MTRequest, partnerId int) {
	if len(responses) == 0 || len(requests) == 0 {
		s.logger.Warn("Empty responses or requests for batch blacklisted user processing")
		return
	}

	startTime := time.Now()
	s.logger.Info("Starting batch processing of blacklisted users",
		zap.Int("totalResponses", len(responses)),
		zap.Int("totalRequests", len(requests)))

	// Group blacklisted responses and their corresponding requests
	var blacklistedTasks []struct {
		response *domain.MTResponse
		request  domain.MTRequest
	}

	for i, response := range responses {
		if response.Code == ResponseCodeBlacklisted {
			if i < len(requests) {
				blacklistedTasks = append(blacklistedTasks, struct {
					response *domain.MTResponse
					request  domain.MTRequest
				}{response, requests[i]})
			}
		}
	}

	if len(blacklistedTasks) == 0 {
		s.logger.Info("No blacklisted users found in batch")
		return
	}

	s.logger.Info("Processing blacklisted users in batch",
		zap.Int("blacklistedCount", len(blacklistedTasks)))

	// Process blacklisted users concurrently
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10) // Limit concurrency

	for _, task := range blacklistedTasks {
		wg.Add(1)
		go func(response *domain.MTResponse, request domain.MTRequest) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			s.handleBlacklistedUserEnhanced(
				request.UserIdentifier,
				request.ProductID,
				response.RequestID,
				partnerId,
				response,
			)
		}(task.response, task.request)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	duration := time.Since(startTime)
	s.logger.Info("Completed batch processing of blacklisted users",
		zap.Int("processedCount", len(blacklistedTasks)),
		zap.Duration("duration", duration))
}

// isBlacklistedResponse checks if a response indicates a BLACKLISTED status
func (s *SubscriptionService) isBlacklistedResponse(response *domain.MTResponse) bool {
	return response.Code == ResponseCodeBlacklisted
}
