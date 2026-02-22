package handler

import (
	"encoding/json"
	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription/internal/domain"
	"github.com/seidu626/subscription-manager/subscription/internal/service"
	"github.com/valyala/fasthttp"
	"log"
	"strconv"
)

type SubscriptionHandler struct {
	service *service.SubscriptionService
	config  *config.Config
}

func NewSubscriptionHandler(service *service.SubscriptionService, c *config.Config) *SubscriptionHandler {
	return &SubscriptionHandler{service: service, config: c}
}

func (h *SubscriptionHandler) ListSubscriptions(ctx *fasthttp.RequestCtx) {
	log.Println("Processing subscription list request")

	// Extract query parameters
	queryParams := map[string]string{
		"startDate":      string(ctx.QueryArgs().Peek("startDate")),
		"endDate":        string(ctx.QueryArgs().Peek("endDate")),
		"productId":      string(ctx.QueryArgs().Peek("productId")),
		"shortcode":      string(ctx.QueryArgs().Peek("shortcode")),
		"userIdentifier": string(ctx.QueryArgs().Peek("userIdentifier")),
		"entryChannel":   string(ctx.QueryArgs().Peek("entryChannel")),
		"page":           string(ctx.QueryArgs().Peek("page")),
		"pageSize":       string(ctx.QueryArgs().Peek("pageSize")),
	}

	// Pass queryParams to the service layer
	listResponse, err := h.service.GetSubscriptions(queryParams)
	if err != nil {
		log.Println(err)
		ctx.Error("Error fetching listResponse", fasthttp.StatusInternalServerError)
		return
	}

	// Marshal the listResponse to JSON
	response, err := json.Marshal(listResponse)
	if err != nil {
		ctx.Error("Error formatting response", fasthttp.StatusInternalServerError)
		return
	}

	// Prepare pagination data to be added in headers
	paginationData := struct {
		Page        int  `json:"page"`
		PageSize    int  `json:"pageSize"`
		TotalCount  int  `json:"totalCount"`
		TotalPages  int  `json:"totalPages"`
		HasNextPage bool `json:"hasNextPage"`
		HasPrevPage bool `json:"hasPrevPage"`
	}{
		Page:        listResponse.Page,
		PageSize:    listResponse.PageSize,
		TotalCount:  listResponse.TotalCount,
		TotalPages:  listResponse.TotalPages,
		HasNextPage: listResponse.HasNextPage,
		HasPrevPage: listResponse.HasPrevPage,
	}

	// Convert pagination data to JSON and set in the X-Pagination header
	paginationJSON, err := json.Marshal(paginationData)
	if err != nil {
		log.Println("Error marshalling pagination data:", err)
		ctx.Error("Error formatting pagination data", fasthttp.StatusInternalServerError)
		return
	}
	ctx.Response.Header.Set("X-Pagination", string(paginationJSON))

	// Set response headers and body
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBody(response)
}

func (h *SubscriptionHandler) handleSubscription(ctx *fasthttp.RequestCtx, subscriptionType string) {
	log.Printf("Processing subscription request: %s", ctx.Request.String())

	// Extract partnerRoleId from the context
	partnerRoleIdStr := ctx.UserValue("partnerRoleId").(string)
	partnerRoleId, err := strconv.Atoi(partnerRoleIdStr)
	if err != nil {
		log.Printf("Error converting partnerRoleId: %v", err)
		writeJSONResponse(ctx, fasthttp.StatusBadRequest, map[string]interface{}{
			"message": "Invalid partnerRoleId",
			"code":    "FAILURE",
			"inError": true,
		})
		return
	}

	// Confirm path is deprecated in subscription-partner.
	// Real confirm is handled by subscription-external's Partner API.
	if subscriptionType == "CONFIRM" {
		writeJSONResponse(ctx, fasthttp.StatusNotImplemented, map[string]interface{}{
			"message": "Confirmation is handled by /api/external/v1/subscription/optin/confirm",
			"code":    "NOT_SUPPORTED",
			"inError": true,
		})
		return
	}

	// Initialize the subscription request based on the type
	var subscriptionRequest interface{}
	switch subscriptionType {
	case "OPTIN":
		subscriptionRequest = &domain.SubscriptionRequest{}
	case "CONFIRM":
		subscriptionRequest = &domain.SubscriptionConfirmationRequest{}
	case "OPTOUT":
		subscriptionRequest = &domain.UnsubscriptionRequest{}
	case "STATUS":
		subscriptionRequest = &domain.GetStatusRequest{}
	default:
		writeJSONResponse(ctx, fasthttp.StatusBadRequest, map[string]interface{}{
			"message": "Invalid subscription type",
			"code":    "FAILURE",
			"inError": true,
		})
		return
	}

	// Unmarshal the request body into the appropriate struct
	if err := json.Unmarshal(ctx.PostBody(), subscriptionRequest); err != nil {
		log.Printf("Error unmarshalling request: %v", err)
		writeJSONResponse(ctx, fasthttp.StatusBadRequest, map[string]interface{}{
			"message": "Invalid request payload",
			"code":    "FAILURE",
			"inError": true,
		})
		return
	}

	// Set partnerRoleId in the request struct dynamically based on its type
	switch req := subscriptionRequest.(type) {
	case *domain.SubscriptionRequest:
		req.PartnerRoleId = partnerRoleId
	case *domain.SubscriptionConfirmationRequest:
		req.PartnerRoleId = partnerRoleId
	case *domain.UnsubscriptionRequest:
		req.PartnerRoleId = partnerRoleId
	case *domain.GetStatusRequest:
		req.PartnerRoleId = partnerRoleId
	}

	// Process the subscription action based on type
	var processErr error
	switch subscriptionType {
	case "OPTIN":
		processErr = h.service.ProcessOptin(subscriptionRequest.(*domain.SubscriptionRequest))
	case "CONFIRM":
		processErr = h.service.ProcessConfirmation(subscriptionRequest.(*domain.SubscriptionConfirmationRequest))
	case "OPTOUT":
		processErr = h.service.ProcessOptout(subscriptionRequest.(*domain.UnsubscriptionRequest))
	case "STATUS":
		status, processErr := h.service.ProcessStatus(subscriptionRequest.(*domain.GetStatusRequest))
		if status != nil && processErr == nil {
			body, err := json.Marshal(status)
			if err == nil {
				ctx.SetBody(body)
			}
			log.Printf("Error marshalling status: %v", err)
			processErr = err
		}

	}

	if processErr != nil {
		log.Printf("Error processing subscription: %+v", processErr)
		writeJSONResponse(ctx, fasthttp.StatusInternalServerError, map[string]interface{}{
			"message": "Error processing subscription",
			"code":    "FAILURE",
			"inError": true,
		})
		return
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
	if subscriptionType != "STATUS" {
		writeJSONResponse(ctx, fasthttp.StatusOK, map[string]interface{}{
			"message": "Subscription processed successfully",
			"code":    "SUCCESS",
			"inError": false,
		})
	}
}

func (h *SubscriptionHandler) OptinHandler(ctx *fasthttp.RequestCtx) {
	h.handleSubscription(ctx, "OPTIN")
}

func (h *SubscriptionHandler) ConfirmHandler(ctx *fasthttp.RequestCtx) {
	h.handleSubscription(ctx, "CONFIRM")
}

func (h *SubscriptionHandler) OptoutHandler(ctx *fasthttp.RequestCtx) {
	h.handleSubscription(ctx, "OPTOUT")
}

func (h *SubscriptionHandler) StatusHandler(ctx *fasthttp.RequestCtx) {
	h.handleSubscription(ctx, "STATUS")
}

func writeJSONResponse(ctx *fasthttp.RequestCtx, status int, payload interface{}) {
	body, err := json.Marshal(payload)
	if err != nil {
		ctx.Error("Error formatting response", fasthttp.StatusInternalServerError)
		return
	}
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(status)
	ctx.SetBody(body)
}
