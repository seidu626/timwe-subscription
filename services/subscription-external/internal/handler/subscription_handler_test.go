package handler

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func TestOptinHandler_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		serviceError   error
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "MTResponseError - INVALID_MSISDN",
			serviceError: &domain.MTResponseError{
				Code:    "SUCCESS",
				Message: "Invalid MSISDN",
				Details: map[string]interface{}{
					"subscriptionResult": "INVALID_MSISDN",
				},
			},
			expectedStatus: fasthttp.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"status":  "error",
				"message": "Invalid MSISDN",
				"code":    "SUCCESS",
				"details": map[string]interface{}{
					"subscriptionResult": "INVALID_MSISDN",
				},
			},
		},
		{
			name: "MTResponseError - OPTIN_CONFIG_NOT_FOUND",
			serviceError: &domain.MTResponseError{
				Code:    "SUCCESS",
				Message: "Optin configuration not found!",
				Details: map[string]interface{}{
					"subscriptionResult": "OPTIN_CONFIG_NOT_FOUND",
				},
			},
			expectedStatus: fasthttp.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"status":  "error",
				"message": "Optin configuration not found!",
				"code":    "SUCCESS",
				"details": map[string]interface{}{
					"subscriptionResult": "OPTIN_CONFIG_NOT_FOUND",
				},
			},
		},
		{
			name:           "Generic Error",
			serviceError:   errors.New("generic error"),
			expectedStatus: fasthttp.StatusBadRequest,
			expectedBody:   nil, // Generic error returns plain text
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			req := domain.OptinRequest{
				Telco:        "test",
				Msisdn:       "1234567890",
				EntryChannel: "test",
			}
			reqBody, _ := json.Marshal(req)

			// Create fasthttp context
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.SetRequestURI("/api/v1/subscription-external")
			ctx.Request.Header.SetMethod("POST")
			ctx.Request.SetBody(reqBody)

			// Create handler with mock service override
			h := &SubscriptionHandler{logger: zap.NewNop(), service: &service.SubscriptionService{}}
			h.processOptinFn = func(*domain.OptinRequest) error { return tt.serviceError }

			// Call the handler
			h.OptinHandler(ctx)

			// Assert status code
			assert.Equal(t, tt.expectedStatus, ctx.Response.StatusCode())

			// Assert response body
			if tt.expectedBody != nil {
				var responseBody map[string]interface{}
				err := json.Unmarshal(ctx.Response.Body(), &responseBody)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedBody, responseBody)
			}
		})
	}
}
