package handler

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// PostbackAdminHandler exposes postback diagnostics for admins.
// This is useful because postbacks are server-to-server and won't appear in browser Network logs.
type PostbackAdminHandler struct {
	repo   *repository.PostbackRepository
	logger *zap.Logger
}

func NewPostbackAdminHandler(repo *repository.PostbackRepository, logger *zap.Logger) *PostbackAdminHandler {
	return &PostbackAdminHandler{repo: repo, logger: logger}
}

// GetByTransactionID handles:
// GET /v1/admin/postbacks?transaction_id=<uuid>
func (h *PostbackAdminHandler) GetByTransactionID(ctx *fasthttp.RequestCtx) {
	raw := string(ctx.QueryArgs().Peek("transaction_id"))
	if raw == "" {
		ctx.Error("transaction_id is required", fasthttp.StatusBadRequest)
		return
	}

	txID, err := uuid.Parse(raw)
	if err != nil {
		ctx.Error("transaction_id must be a valid UUID", fasthttp.StatusBadRequest)
		return
	}

	outbox, err := h.repo.GetOutboxByTransactionID(txID)
	if err != nil {
		h.logger.Error("Failed to fetch postback outbox", zap.String("transaction_id", txID.String()), zap.Error(err))
		ctx.Error("Failed to fetch postbacks", fasthttp.StatusInternalServerError)
		return
	}

	outboxIDs := make([]uuid.UUID, 0, len(outbox))
	for _, o := range outbox {
		outboxIDs = append(outboxIDs, o.ID)
	}

	attemptsByOutbox, err := h.repo.GetAttemptsByOutboxIDs(outboxIDs)
	if err != nil {
		h.logger.Error("Failed to fetch postback attempts", zap.String("transaction_id", txID.String()), zap.Error(err))
		ctx.Error("Failed to fetch postback attempts", fasthttp.StatusInternalServerError)
		return
	}

	type outboxWithAttempts struct {
		Outbox   any   `json:"outbox"`
		Attempts any   `json:"attempts"`
	}

	entries := make([]outboxWithAttempts, 0, len(outbox))
	for _, o := range outbox {
		entries = append(entries, outboxWithAttempts{
			Outbox:   o,
			Attempts: attemptsByOutbox[o.ID],
		})
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(map[string]any{
		"transaction_id": txID.String(),
		"count":          len(entries),
		"entries":        entries,
	})
}

func (h *PostbackAdminHandler) String() string {
	return "PostbackAdminHandler"
}

