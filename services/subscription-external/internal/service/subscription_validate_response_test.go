package service

import (
	"testing"

	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"go.uber.org/zap"
)

func TestValidateMTResponse_AllowsOptinPreactiveWaitConf(t *testing.T) {
	cfg := &config.Config{}
	cfg.Application.TIMWE.PartnerRoleID = "2117"

	service := &SubscriptionService{
		logger: zap.NewNop(),
		config: cfg,
	}

	response := &domain.MTResponse{
		Code:    ResponseCodeSuccess,
		InError: false,
		ResponseData: map[string]interface{}{
			"transactionId":      "tx-preactive-1",
			"subscriptionResult": SubscriptionResultOptinPreactiveWaitConf,
		},
	}

	if err := service.validateMTResponse(response, domain.MTRequest{
		UserIdentifier: "233241234567",
		ProductID:      8509,
	}); err != nil {
		t.Fatalf("expected no error for OPTIN_PREACTIVE_WAIT_CONF, got %v", err)
	}
}
