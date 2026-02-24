package service

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/utils"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

type adminExecutionResult struct {
	requestHeaders    map[string]string
	requestBody       json.RawMessage
	requestTimestamp  time.Time
	responseStatus    int
	responseHeaders   map[string]string
	responseBody      json.RawMessage
	responseTimestamp *time.Time
	durationMs        int64
	externalTxID      string
	parsedResponse    *domain.MTResponse
}

func ptrFromString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func normalizeHeaders(input map[string]string) map[string]string {
	normalized := make(map[string]string)
	for key, value := range input {
		trimmedKey := strings.ToLower(strings.TrimSpace(key))
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		normalized[trimmedKey] = trimmedValue
	}
	return normalized
}

func headersFromResponse(header *fasthttp.ResponseHeader) map[string]string {
	result := make(map[string]string)
	header.VisitAll(func(key, value []byte) {
		result[strings.ToLower(string(key))] = string(value)
	})
	return result
}

func (s *SubscriptionService) resolveTIMWEAuthKey() (string, error) {
	if len(s.config.Application.TIMWE.AuthenticationKey) > 10 {
		return s.config.Application.TIMWE.AuthenticationKey, nil
	}
	authKey, err := utils.GetCachedAuthKey(s.config.Application.TIMWE.PartnerServiceID, s.config.Application.TIMWE.Psk)
	if err != nil {
		return "", fmt.Errorf("failed to generate auth key: %w", err)
	}
	return authKey, nil
}

func (s *SubscriptionService) resolveAdminPartnerRoleID(requestPartnerRoleID int) (int, error) {
	if requestPartnerRoleID > 0 {
		return requestPartnerRoleID, nil
	}
	partnerRoleID, err := strconv.Atoi(s.config.Application.TIMWE.PartnerRoleID)
	if err != nil {
		return 0, fmt.Errorf("invalid PartnerRoleID: %w", err)
	}
	return partnerRoleID, nil
}

func (s *SubscriptionService) executeAdminTIMWERequest(url string, payload interface{}, authKey, providedExternalTxID string, customHeaders map[string]string, maxRetries int) (*adminExecutionResult, error) {
	requestBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	if maxRetries <= 0 {
		maxRetries = 3
	}
	baseDelay := 200 * time.Millisecond
	normalizedCustomHeaders := normalizeHeaders(customHeaders)
	baseExternalTxID := strings.TrimSpace(providedExternalTxID)
	if baseExternalTxID == "" {
		baseExternalTxID = normalizedCustomHeaders["external-tx-id"]
	}

	var lastResult *adminExecutionResult

	for attempt := 1; attempt <= maxRetries; attempt++ {
		externalTxID := baseExternalTxID
		if externalTxID == "" {
			externalTxID = uuid.NewString()
		}

		headers := map[string]string{
			"apikey":         s.config.Application.TIMWE.APIKey,
			"authentication": authKey,
			"external-tx-id": externalTxID,
			"content-type":   "application/json",
			"accept":         "*/*",
		}
		for key, value := range normalizedCustomHeaders {
			headers[key] = value
		}
		if ext, ok := headers["external-tx-id"]; ok && strings.TrimSpace(ext) != "" {
			externalTxID = strings.TrimSpace(ext)
		} else {
			headers["external-tx-id"] = externalTxID
		}

		req := fasthttp.AcquireRequest()
		res := fasthttp.AcquireResponse()
		requestTime := time.Now().UTC()

		req.SetRequestURI(url)
		req.Header.SetMethod(fasthttp.MethodPost)
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		req.SetBody(requestBody)

		callStart := time.Now()
		callErr := s.client.Do(req, res)
		durationMs := time.Since(callStart).Milliseconds()
		responseTime := time.Now().UTC()

		responseBody := append([]byte(nil), res.Body()...)
		responseHeaders := headersFromResponse(&res.Header)

		lastResult = &adminExecutionResult{
			requestHeaders:    headers,
			requestBody:       json.RawMessage(append([]byte(nil), requestBody...)),
			requestTimestamp:  requestTime,
			responseStatus:    res.StatusCode(),
			responseHeaders:   responseHeaders,
			responseBody:      json.RawMessage(responseBody),
			responseTimestamp: &responseTime,
			durationMs:        durationMs,
			externalTxID:      externalTxID,
		}

		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(res)

		if callErr != nil {
			lastResult.responseStatus = 0
			lastResult.responseHeaders = map[string]string{}
			lastResult.responseBody = json.RawMessage([]byte("null"))
			lastResult.responseTimestamp = nil
			if attempt == maxRetries {
				return lastResult, fmt.Errorf("failed to call TIMWE after %d attempts: %w", maxRetries, callErr)
			}
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			time.Sleep(delay)
			continue
		}

		if lastResult.responseStatus != fasthttp.StatusOK {
			if attempt == maxRetries {
				return lastResult, fmt.Errorf("TIMWE request failed with status code: %d", lastResult.responseStatus)
			}
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			time.Sleep(delay)
			continue
		}

		if len(responseBody) > 0 {
			var mtResponse domain.MTResponse
			if err := json.Unmarshal(responseBody, &mtResponse); err != nil {
				if attempt == maxRetries {
					return lastResult, fmt.Errorf("failed to parse TIMWE response: %w", err)
				}
				delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
				time.Sleep(delay)
				continue
			}
			lastResult.parsedResponse = &mtResponse
			if mtResponse.Code == ResponseCodeInternalError && attempt < maxRetries {
				delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
				time.Sleep(delay)
				continue
			}
		}

		return lastResult, nil
	}

	if lastResult == nil {
		return nil, fmt.Errorf("TIMWE request was not executed")
	}
	return lastResult, nil
}

func buildActionErrorPayload(message, kind string, details map[string]interface{}) json.RawMessage {
	payload := map[string]interface{}{
		"message": message,
		"type":    kind,
	}
	if len(details) > 0 {
		payload["details"] = details
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage([]byte("null"))
	}
	return out
}

func (s *SubscriptionService) buildBusinessErrorPayload(response *domain.MTResponse) json.RawMessage {
	if response == nil {
		return json.RawMessage([]byte("null"))
	}
	if !response.InError && response.Code == ResponseCodeSuccess {
		return json.RawMessage([]byte("null"))
	}
	details := map[string]interface{}{
		"code":      response.Code,
		"inError":   response.InError,
		"requestId": response.RequestID,
		"message":   response.Message,
	}
	if response.ResponseData != nil {
		details["responseData"] = response.ResponseData
	}
	return buildActionErrorPayload("TIMWE returned a non-success response", "timwe_response_error", details)
}

func (s *SubscriptionService) ExecuteAdminSubscriptionAction(operation domain.AdminActionOperation, req domain.AdminSubscriptionActionRequest) (*domain.AdminSubscriptionActionLog, error) {
	actionID := uuid.NewString()
	createdAt := time.Now().UTC()

	partnerRoleID, err := s.resolveAdminPartnerRoleID(req.PartnerRoleID)
	if err != nil {
		return nil, err
	}
	authKey, err := s.resolveTIMWEAuthKey()
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.UserIdentifierType) == "" {
		req.UserIdentifierType = timweDefaultMSISDNType
	}

	var (
		url           string
		payload       interface{}
		validationErr error
	)

	switch operation {
	case domain.AdminActionOptin:
		mtReq := domain.MTRequest{
			ProductID:          req.ProductID,
			MCC:                req.MCC,
			MNC:                req.MNC,
			UserIdentifier:     req.MSISDN,
			UserIdentifierType: req.UserIdentifierType,
			EntryChannel:       req.EntryChannel,
			LargeAccount:       req.LargeAccount,
			SubKeyword:         req.SubKeyword,
			MoTransactionUUID:  req.TrackingID,
			CampaignUrl:        req.CampaignURL,
		}
		payload, err = s.buildTIMWEOptinPayload(mtReq)
		if err != nil {
			return nil, err
		}
		url = fmt.Sprintf("%s/subscription/optin/%d", s.config.Application.TIMWE.BaseURL, partnerRoleID)
		if req.ClientIP != "" {
			payloadMap, mapErr := marshalPayloadToMap(payload)
			if mapErr != nil {
				return nil, mapErr
			}
			payloadMap["clientIp"] = req.ClientIP
			payload = payloadMap
		}
	case domain.AdminActionOptout:
		optoutReq := domain.UnsubscriptionRequest{
			UserIdentifier:        req.MSISDN,
			UserIdentifierType:    req.UserIdentifierType,
			ProductId:             req.ProductID,
			Mcc:                   ptrFromString(req.MCC),
			Mnc:                   ptrFromString(req.MNC),
			EntryChannel:          ptrFromString(req.EntryChannel),
			LargeAccount:          ptrFromString(req.LargeAccount),
			SubKeyword:            ptrFromString(req.SubKeyword),
			TrackingId:            ptrFromString(req.TrackingID),
			ClientIp:              ptrFromString(req.ClientIP),
			ControlKeyword:        req.ControlKeyword,
			ControlServiceKeyword: req.ControlServiceKeyword,
			SubId:                 req.SubID,
			CancelReason:          req.CancelReason,
			CancelSource:          req.CancelSource,
		}
		payload, err = s.buildTIMWEOptoutPayload(optoutReq)
		if err != nil {
			return nil, err
		}
		url = fmt.Sprintf("%s/subscription/optout/%d", s.config.Application.TIMWE.BaseURL, partnerRoleID)
	case domain.AdminActionConfirm:
		confirmReq := domain.SubscriptionConfirmationRequest{
			UserIdentifier:      req.MSISDN,
			UserIdentifierType:  req.UserIdentifierType,
			ProductId:           req.ProductID,
			Mcc:                 ptrFromString(req.MCC),
			Mnc:                 ptrFromString(req.MNC),
			EntryChannel:        ptrFromString(req.EntryChannel),
			ClientIp:            ptrFromString(req.ClientIP),
			TransactionAuthCode: req.TransactionAuthCode,
		}
		payload, err = s.buildTIMWEOptinConfirmPayload(confirmReq)
		if err != nil {
			return nil, err
		}
		url = fmt.Sprintf("%s/subscription/optin/confirm/%d", s.config.Application.TIMWE.BaseURL, partnerRoleID)
	case domain.AdminActionStatus:
		statusReq := domain.GetStatusRequest{
			UserIdentifier:        req.MSISDN,
			UserIdentifierType:    req.UserIdentifierType,
			ProductId:             req.ProductID,
			Mcc:                   ptrFromString(req.MCC),
			Mnc:                   ptrFromString(req.MNC),
			EntryChannel:          ptrFromString(req.EntryChannel),
			ClientIp:              ptrFromString(req.ClientIP),
			ControlKeyword:        req.ControlKeyword,
			ControlServiceKeyword: req.ControlServiceKeyword,
			SubId:                 req.SubID,
		}
		payload, err = s.buildTIMWEStatusPayload(statusReq)
		if err != nil {
			return nil, err
		}
		url = fmt.Sprintf("%s/subscription/status/%d", s.config.Application.TIMWE.BaseURL, partnerRoleID)
	default:
		return nil, fmt.Errorf("unsupported admin action operation: %s", operation)
	}

	execResult, execErr := s.executeAdminTIMWERequest(url, payload, authKey, req.ExternalTxID, req.Headers, 3)
	if execResult == nil {
		return nil, execErr
	}

	serviceResult := json.RawMessage([]byte("null"))
	errorPayload := json.RawMessage([]byte("null"))

	if execErr != nil {
		errorPayload = buildActionErrorPayload(execErr.Error(), "transport_error", map[string]interface{}{
			"statusCode": execResult.responseStatus,
		})
	}

	if execResult.parsedResponse != nil {
		if encoded, err := json.Marshal(execResult.parsedResponse); err == nil {
			serviceResult = json.RawMessage(encoded)
		}

		switch operation {
		case domain.AdminActionOptin:
			mtReq := domain.MTRequest{
				UserIdentifier: req.MSISDN,
				ProductID:      req.ProductID,
			}
			validationErr = s.validateMTResponse(execResult.parsedResponse, mtReq)
		case domain.AdminActionConfirm:
			mtReq := domain.MTRequest{
				UserIdentifier: req.MSISDN,
				ProductID:      req.ProductID,
			}
			validationErr = s.validateMTResponse(execResult.parsedResponse, mtReq)
		default:
			validationErr = nil
		}

		if validationErr != nil {
			errorPayload = buildActionErrorPayload(validationErr.Error(), "validation_error", map[string]interface{}{
				"code":      execResult.parsedResponse.Code,
				"requestId": execResult.parsedResponse.RequestID,
			})
		} else if businessErrorPayload := s.buildBusinessErrorPayload(execResult.parsedResponse); string(businessErrorPayload) != "null" {
			errorPayload = businessErrorPayload
		}
	}

	logEntry := &domain.AdminSubscriptionActionLog{
		ID:                 actionID,
		Operation:          operation,
		MSISDN:             req.MSISDN,
		ProductID:          req.ProductID,
		PartnerRoleID:      partnerRoleID,
		ExternalTxID:       execResult.externalTxID,
		AdminRequestID:     req.AdminRequestID,
		RequestMethod:      fasthttp.MethodPost,
		RequestURL:         url,
		RequestHeaders:     execResult.requestHeaders,
		RequestBody:        execResult.requestBody,
		RequestTimestamp:   execResult.requestTimestamp,
		ResponseStatusCode: execResult.responseStatus,
		ResponseHeaders:    execResult.responseHeaders,
		ResponseBody:       execResult.responseBody,
		ResponseTimestamp:  execResult.responseTimestamp,
		ServiceResult:      serviceResult,
		ErrorPayload:       errorPayload,
		DurationMs:         execResult.durationMs,
		CreatedAt:          createdAt,
	}

	s.logger.Info("Admin subscription action executed",
		zap.String("actionId", logEntry.ID),
		zap.String("operation", string(logEntry.Operation)),
		zap.String("msisdn", logEntry.MSISDN),
		zap.Int("productId", logEntry.ProductID),
		zap.String("externalTxId", logEntry.ExternalTxID),
		zap.Any("requestHeaders", logEntry.RequestHeaders),
		zap.Any("requestBody", string(logEntry.RequestBody)),
		zap.Int("responseStatusCode", logEntry.ResponseStatusCode),
		zap.Any("responseHeaders", logEntry.ResponseHeaders),
		zap.Any("responseBody", string(logEntry.ResponseBody)),
		zap.Any("serviceResult", string(logEntry.ServiceResult)),
		zap.Any("errorPayload", string(logEntry.ErrorPayload)),
	)

	return logEntry, execErr
}

func marshalPayloadToMap(payload interface{}) (map[string]interface{}, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	result := make(map[string]interface{})
	if err := json.Unmarshal(encoded, &result); err != nil {
		return nil, err
	}
	return result, nil
}
