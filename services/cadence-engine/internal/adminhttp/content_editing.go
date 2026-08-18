package adminhttp

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/seidu626/subscription-manager/cadence-engine/internal/domain"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"go.uber.org/zap"
)

// requestActor identifies the acting admin for the content revision audit
// trail; empty when the request carries no user identity (static token).
func requestActor(r *http.Request) string {
	identity, _ := tenantctx.FromContext(r.Context())
	return firstNonBlank(identity.Email, identity.Subject)
}

// handleContentSubroute dispatches /series/{id}/content/{impact|clone|itemID}.
func (s *Server) handleContentSubroute(w http.ResponseWriter, r *http.Request, series *domain.MessageSeries, sub string) {
	switch sub {
	case "impact":
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleContentImpact(w, r, series)
	case "clone":
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleContentClone(w, r, series)
	default:
		itemID, err := strconv.ParseInt(sub, 10, 64)
		if err != nil || itemID <= 0 {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPatch {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleContentItemPatch(w, r, series, itemID)
	}
}

// handleContentImpact reports the blast radius of editing a content version:
// GET /v1/admin/cadence/series/{id}/content/impact?contentVersion=N
func (s *Server) handleContentImpact(w http.ResponseWriter, r *http.Request, series *domain.MessageSeries) {
	contentVersion := series.ContentVersion
	if v := strings.TrimSpace(r.URL.Query().Get("contentVersion")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "invalid contentVersion")
			return
		}
		contentVersion = n
	}

	activeStates, pendingJobs, err := s.repo.ContentImpact(r.Context(), series.ID, contentVersion)
	if err != nil {
		s.logger.Error("content impact failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to compute content impact")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"series_id":       series.ID,
		"content_version": contentVersion,
		"is_live":         contentVersion == series.ContentVersion,
		"active_states":   activeStates,
		"pending_jobs":    pendingJobs,
	})
}

// handleContentClone copies a whole content version to a new draft version:
// POST /v1/admin/cadence/series/{id}/content/clone
// Body: { "from_version": N, "to_version": M } (to_version optional; defaults
// to max(version)+1). Refuses to clone into a version that already has items.
func (s *Server) handleContentClone(w http.ResponseWriter, r *http.Request, series *domain.MessageSeries) {
	var req struct {
		FromVersion int `json:"from_version"`
		ToVersion   int `json:"to_version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if req.FromVersion <= 0 {
		writeError(w, http.StatusBadRequest, "from_version is required and must be > 0")
		return
	}

	sourceCount, err := s.repo.CountContentItems(r.Context(), series.ID, req.FromVersion)
	if err != nil {
		s.logger.Error("count content failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to clone version")
		return
	}
	if sourceCount == 0 {
		writeError(w, http.StatusBadRequest, "from_version has no content items")
		return
	}

	toVersion := req.ToVersion
	if toVersion <= 0 {
		maxVersion, err := s.repo.MaxContentVersion(r.Context(), series.ID)
		if err != nil {
			s.logger.Error("max content version failed", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to clone version")
			return
		}
		toVersion = maxVersion + 1
	}
	if toVersion == req.FromVersion {
		writeError(w, http.StatusBadRequest, "to_version must differ from from_version")
		return
	}
	targetCount, err := s.repo.CountContentItems(r.Context(), series.ID, toVersion)
	if err != nil {
		s.logger.Error("count content failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to clone version")
		return
	}
	if targetCount > 0 {
		writeError(w, http.StatusConflict, "to_version already has content items")
		return
	}

	tx, err := s.repo.BeginTx(r.Context())
	if err != nil {
		s.logger.Error("begin tx failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to clone version")
		return
	}
	defer func() { _ = tx.Rollback() }()

	copied, err := s.repo.CloneContentVersionTx(r.Context(), tx, series.ID, req.FromVersion, toVersion)
	if err != nil {
		s.logger.Error("clone version failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to clone version")
		return
	}
	if err := tx.Commit(); err != nil {
		s.logger.Error("commit failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to clone version")
		return
	}

	s.logger.Info("cloned content version",
		zap.Int64("series_id", series.ID),
		zap.Int("from_version", req.FromVersion),
		zap.Int("to_version", toVersion),
		zap.Int64("items_copied", copied),
	)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"series_id":    series.ID,
		"from_version": req.FromVersion,
		"to_version":   toVersion,
		"items_copied": copied,
	})
}

// handleContentItemPatch edits a content item in place by id:
// PATCH /v1/admin/cadence/series/{id}/content/{itemId}
// Body fields are all optional: message_text, content_kind, link_url,
// cta_label, is_active. Version and seq_no are immutable (renumbering a
// published version would skip or repeat messages for mid-series cursors).
func (s *Server) handleContentItemPatch(w http.ResponseWriter, r *http.Request, series *domain.MessageSeries, itemID int64) {
	var req struct {
		MessageText *string `json:"message_text"`
		ContentKind *string `json:"content_kind"`
		LinkURL     *string `json:"link_url"`
		CTALabel    *string `json:"cta_label"`
		IsActive    *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	existing, err := s.repo.GetContentItemForSeries(r.Context(), series.ID, itemID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "content item not found")
			return
		}
		s.logger.Error("get content item failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to load content item")
		return
	}

	messageText := existing.MessageText
	if req.MessageText != nil {
		messageText = strings.TrimSpace(*req.MessageText)
		if messageText == "" {
			writeError(w, http.StatusBadRequest, "message_text must not be empty")
			return
		}
	}

	kind := existing.ContentKind
	if req.ContentKind != nil {
		kind = *req.ContentKind
	}
	var linkURL, ctaLabel *string
	if strings.EqualFold(strings.TrimSpace(kind), "LINK") {
		linkURL = existing.LinkURL
		if req.LinkURL != nil {
			linkURL = trimStringPtr(req.LinkURL)
		}
		ctaLabel = existing.CTALabel
		if req.CTALabel != nil {
			ctaLabel = trimStringPtr(req.CTALabel)
		}
	}
	contentKind, linkURL, ctaLabel, err := validateContentFields(kind, linkURL, ctaLabel)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	isActive := existing.IsActive
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	tx, err := s.repo.BeginTx(r.Context())
	if err != nil {
		s.logger.Error("begin tx failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to update content item")
		return
	}
	defer func() { _ = tx.Rollback() }()

	if !isActive && existing.IsActive && existing.ContentVersion == series.ContentVersion {
		others, err := s.repo.CountOtherActiveContentItemsTx(r.Context(), tx, series.ID, existing.ContentVersion, itemID)
		if err != nil {
			s.logger.Error("count active content failed", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to update content item")
			return
		}
		if others == 0 {
			writeError(w, http.StatusConflict, "cannot deactivate the last active item of the live version; publish another version first")
			return
		}
	}

	if actor := requestActor(r); actor != "" {
		if err := s.repo.SetTxActor(r.Context(), tx, actor); err != nil {
			s.logger.Warn("set tx actor failed", zap.Error(err))
		}
	}
	if err := s.repo.UpdateContentItemTx(r.Context(), tx, itemID, messageText, contentKind, linkURL, ctaLabel, isActive); err != nil {
		s.logger.Error("update content item failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to update content item")
		return
	}
	if err := tx.Commit(); err != nil {
		s.logger.Error("commit failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to update content item")
		return
	}

	updated, err := s.repo.GetContentItemForSeries(r.Context(), series.ID, itemID)
	if err != nil {
		s.logger.Error("reload content item failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to reload content item")
		return
	}
	s.logger.Info("content item updated",
		zap.Int64("series_id", series.ID),
		zap.Int64("content_item_id", itemID),
		zap.Int("content_version", updated.ContentVersion),
		zap.Int("seq_no", updated.SeqNo),
		zap.Bool("live_version", updated.ContentVersion == series.ContentVersion),
	)
	writeJSON(w, http.StatusOK, updated)
}
