package adminhttp

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/cadence-engine/internal/domain"
	"github.com/seidu626/subscription-manager/cadence-engine/internal/repository"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"go.uber.org/zap"
)

type Config struct {
	Addr string
}

type Server struct {
	cfg    Config
	logger *zap.Logger
	repo   *repository.CadenceRepository

	access *access
	http   *http.Server
}

func NewServer(repo *repository.CadenceRepository, logger *zap.Logger, cfg Config) *Server {
	s := &Server{
		cfg:    cfg,
		logger: logger,
		repo:   repo,
		access: newAccess(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/admin/cadence/series", s.handleSeries)                // GET, POST
	mux.HandleFunc("/v1/admin/cadence/series/", s.handleSeriesByID)           // GET, PATCH + nested
	mux.HandleFunc("/v1/admin/cadence/content/import/csv", s.handleCSVImport) // POST

	s.http = &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		s.logger.Info("cadence admin http listening", zap.String("addr", s.cfg.Addr))
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.http.Shutdown(shutdownCtx)
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		s.access.setCORS(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleSeries(w http.ResponseWriter, r *http.Request) {
	if s.access.handlePreflight(w, r) {
		return
	}
	if !s.access.require(w, r) {
		return
	}
	tenantID, channelID, ok := s.tenantScope(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		var partnerRoleID *int
		if v := strings.TrimSpace(q.Get("partnerRoleId")); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				writeError(w, http.StatusBadRequest, "invalid partnerRoleId")
				return
			}
			partnerRoleID = &n
		}
		var productID *int
		if v := strings.TrimSpace(q.Get("productId")); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				writeError(w, http.StatusBadRequest, "invalid productId")
				return
			}
			productID = &n
		}
		var active *bool
		if v := strings.TrimSpace(q.Get("active")); v != "" {
			b, err := parseBool(v)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid active")
				return
			}
			active = &b
		}
		limit := 200
		if v := strings.TrimSpace(q.Get("limit")); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid limit")
				return
			}
			limit = n
		}

		items, err := s.repo.ListSeries(r.Context(), tenantID, channelID, partnerRoleID, productID, active, limit)
		if err != nil {
			s.logger.Error("list series failed", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to list series")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"series": items})
		return
	case http.MethodPost:
		var req struct {
			PartnerRoleID  int    `json:"partner_role_id"`
			ProductID      int    `json:"product_id"`
			ChannelID      string `json:"channel_id"`
			Name           string `json:"name"`
			Mode           string `json:"mode"`
			ContentVersion int    `json:"content_version"`
			IsActive       *bool  `json:"is_active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.Mode = strings.TrimSpace(req.Mode)
		if req.PartnerRoleID <= 0 || req.ProductID <= 0 || req.Name == "" {
			writeError(w, http.StatusBadRequest, "partner_role_id, product_id, name are required")
			return
		}
		if req.Mode == "" {
			req.Mode = "SEQUENTIAL"
		}
		active := true
		if req.IsActive != nil {
			active = *req.IsActive
		}

		series, err := s.repo.UpsertSeries(r.Context(), tenantID, firstNonBlank(req.ChannelID, channelID), req.PartnerRoleID, req.ProductID, req.Name, req.Mode, req.ContentVersion, active)
		if err != nil {
			s.logger.Error("upsert series failed", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to upsert series")
			return
		}
		writeJSON(w, http.StatusOK, series)
		return
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
}

func (s *Server) handleSeriesByID(w http.ResponseWriter, r *http.Request) {
	if s.access.handlePreflight(w, r) {
		return
	}
	if !s.access.require(w, r) {
		return
	}

	// Expected:
	// /v1/admin/cadence/series/{id}
	// /v1/admin/cadence/series/{id}/rule
	// /v1/admin/cadence/series/{id}/content
	path := r.URL.Path
	parts := splitPath(path)
	if len(parts) < 5 { // ["v1","admin","cadence","series","{id}",...]
		http.NotFound(w, r)
		return
	}

	seriesID, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil || seriesID <= 0 {
		http.Error(w, "invalid series id", http.StatusBadRequest)
		return
	}
	tenantID, channelID, ok := s.tenantScope(w, r)
	if !ok {
		return
	}
	series, err := s.repo.GetSeriesForTenant(r.Context(), tenantID, seriesID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "series not found")
			return
		}
		s.logger.Error("get series failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to get series")
		return
	}
	if channelID != "" && !seriesMatchesChannel(series, channelID) {
		writeError(w, http.StatusNotFound, "series not found")
		return
	}

	// Nested routes
	if len(parts) >= 6 {
		switch parts[5] {
		case "rule":
			switch r.Method {
			case http.MethodGet:
				rule, err := s.repo.GetScheduleRule(r.Context(), seriesID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						writeError(w, http.StatusNotFound, "rule not found")
						return
					}
					s.logger.Error("get rule failed", zap.Error(err))
					writeError(w, http.StatusInternalServerError, "failed to get rule")
					return
				}
				writeJSON(w, http.StatusOK, rule)
				return
			case http.MethodPut:
				var req struct {
					RuleKind      string `json:"rule_kind"`
					PreferredTime string `json:"preferred_time"` // HH:MM or HH:MM:SS
					DaysOfWeek    int    `json:"days_of_week"`
					NDays         int    `json:"n_days"`
					SendStartTime string `json:"send_start_time"` // HH:MM
					SendEndTime   string `json:"send_end_time"`   // HH:MM
					Timezone      string `json:"timezone"`
					MaxPerDay     int    `json:"max_per_day"`
					CatchupMode   string `json:"catchup_mode"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				req.RuleKind = strings.TrimSpace(req.RuleKind)
				req.Timezone = strings.TrimSpace(req.Timezone)
				req.CatchupMode = strings.TrimSpace(req.CatchupMode)
				if req.RuleKind == "" || req.PreferredTime == "" || req.SendStartTime == "" || req.SendEndTime == "" || req.Timezone == "" || req.MaxPerDay <= 0 {
					writeError(w, http.StatusBadRequest, "missing required rule fields")
					return
				}

				preferred, err := parseClock(req.PreferredTime)
				if err != nil {
					writeError(w, http.StatusBadRequest, "invalid preferred_time")
					return
				}
				start, err := parseClock(req.SendStartTime)
				if err != nil {
					writeError(w, http.StatusBadRequest, "invalid send_start_time")
					return
				}
				end, err := parseClock(req.SendEndTime)
				if err != nil {
					writeError(w, http.StatusBadRequest, "invalid send_end_time")
					return
				}

				rule := domain.ScheduleRule{
					SeriesID:      seriesID,
					RuleKind:      req.RuleKind,
					PreferredTime: preferred,
					DaysOfWeek:    req.DaysOfWeek,
					NDays:         req.NDays,
					SendStartTime: start,
					SendEndTime:   end,
					Timezone:      req.Timezone,
					MaxPerDay:     req.MaxPerDay,
					CatchupMode:   req.CatchupMode,
				}
				if err := s.repo.UpsertScheduleRule(r.Context(), rule); err != nil {
					s.logger.Error("upsert rule failed", zap.Error(err))
					writeError(w, http.StatusInternalServerError, "failed to upsert rule")
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
				return
			default:
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				return
			}
		case "publish":
			// POST /v1/admin/cadence/series/{id}/publish
			// Body: { "content_version": N }
			// Validates version has active content items, then updates series.content_version
			if r.Method != http.MethodPost {
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				ContentVersion int `json:"content_version"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			if req.ContentVersion <= 0 {
				writeError(w, http.StatusBadRequest, "content_version is required and must be > 0")
				return
			}
			// Validate content items exist for this version
			v := req.ContentVersion
			activeOnly := true
			items, err := s.repo.ListContentItems(r.Context(), seriesID, &v, &activeOnly, 1)
			if err != nil {
				s.logger.Error("list content failed", zap.Error(err))
				writeError(w, http.StatusInternalServerError, "failed to validate content")
				return
			}
			if len(items) == 0 {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("no active content items found for version %d", req.ContentVersion))
				return
			}
			// Update series content_version
			if err := s.repo.PatchSeries(r.Context(), seriesID, nil, nil, &req.ContentVersion); err != nil {
				s.logger.Error("patch series failed", zap.Error(err))
				writeError(w, http.StatusInternalServerError, "failed to publish version")
				return
			}
			s.logger.Info("published content version",
				zap.Int64("series_id", seriesID),
				zap.String("series_name", series.Name),
				zap.Int("old_version", series.ContentVersion),
				zap.Int("new_version", req.ContentVersion),
			)
			writeJSON(w, http.StatusOK, map[string]any{
				"status":            "ok",
				"series_id":         seriesID,
				"previous_version":  series.ContentVersion,
				"published_version": req.ContentVersion,
			})
			return
		case "content":
			switch r.Method {
			case http.MethodGet:
				q := r.URL.Query()
				var contentVersion *int
				if v := strings.TrimSpace(q.Get("contentVersion")); v != "" {
					n, err := strconv.Atoi(v)
					if err != nil || n <= 0 {
						writeError(w, http.StatusBadRequest, "invalid contentVersion")
						return
					}
					contentVersion = &n
				}
				var active *bool
				if v := strings.TrimSpace(q.Get("active")); v != "" {
					b, err := parseBool(v)
					if err != nil {
						writeError(w, http.StatusBadRequest, "invalid active")
						return
					}
					active = &b
				}
				limit := 500
				if v := strings.TrimSpace(q.Get("limit")); v != "" {
					n, err := strconv.Atoi(v)
					if err != nil {
						writeError(w, http.StatusBadRequest, "invalid limit")
						return
					}
					limit = n
				}
				items, err := s.repo.ListContentItems(r.Context(), seriesID, contentVersion, active, limit)
				if err != nil {
					s.logger.Error("list content failed", zap.Error(err))
					writeError(w, http.StatusInternalServerError, "failed to list content")
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"items": items})
				return
			case http.MethodPost:
				var req struct {
					ContentVersion int    `json:"content_version"`
					SeqNo          int    `json:"seq_no"`
					MessageText    string `json:"message_text"`
					IsActive       *bool  `json:"is_active"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					writeError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				req.MessageText = strings.TrimSpace(req.MessageText)
				if req.ContentVersion <= 0 || req.SeqNo <= 0 || req.MessageText == "" {
					writeError(w, http.StatusBadRequest, "content_version, seq_no, message_text are required")
					return
				}
				active := true
				if req.IsActive != nil {
					active = *req.IsActive
				}

				tx, err := s.repo.BeginTx(r.Context())
				if err != nil {
					s.logger.Error("begin tx failed", zap.Error(err))
					writeError(w, http.StatusInternalServerError, "failed to save content")
					return
				}
				defer func() { _ = tx.Rollback() }()

				if _, err := s.repo.UpsertContentItemTx(r.Context(), tx, tenantID, seriesChannelID(series, channelID), seriesID, req.ContentVersion, req.SeqNo, req.MessageText, active); err != nil {
					s.logger.Error("upsert content failed", zap.Error(err))
					writeError(w, http.StatusInternalServerError, "failed to save content")
					return
				}
				if err := tx.Commit(); err != nil {
					s.logger.Error("commit failed", zap.Error(err))
					writeError(w, http.StatusInternalServerError, "failed to save content")
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
				return
			default:
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				return
			}
		default:
			http.NotFound(w, r)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, series)
		return
	case http.MethodPatch:
		var req struct {
			IsActive       *bool   `json:"is_active"`
			Mode           *string `json:"mode"`
			ContentVersion *int    `json:"content_version"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if err := s.repo.PatchSeries(r.Context(), seriesID, req.IsActive, req.Mode, req.ContentVersion); err != nil {
			s.logger.Error("patch series failed", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to patch series")
			return
		}
		series, err := s.repo.GetSeriesForTenant(r.Context(), tenantID, seriesID)
		if err != nil {
			s.logger.Error("get series after patch failed", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to get series")
			return
		}
		writeJSON(w, http.StatusOK, series)
		return
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
}

func (s *Server) handleCSVImport(w http.ResponseWriter, r *http.Request) {
	if s.access.handlePreflight(w, r) {
		return
	}
	if !s.access.require(w, r) {
		return
	}
	tenantID, channelID, ok := s.tenantScope(w, r)
	if !ok {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	dryRun := false
	if v := strings.TrimSpace(r.URL.Query().Get("dryRun")); v != "" {
		b, err := parseBool(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid dryRun")
			return
		}
		dryRun = b
	}

	const maxCSVBytes = int64(10 << 20) // 10MB
	r.Body = http.MaxBytesReader(w, r.Body, maxCSVBytes)
	if err := r.ParseMultipartForm(maxCSVBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file")
		return
	}
	defer func() { _ = f.Close() }()

	importReq, parseErrs := parseCSVImport(f)
	if len(parseErrs) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"errors": parseErrs,
		})
		return
	}

	if dryRun {
		writeJSON(w, http.StatusOK, map[string]any{
			"dry_run":      true,
			"series_count": len(importReq.Series),
			"row_count":    importReq.RowCount,
		})
		return
	}

	tx, err := s.repo.BeginTx(r.Context())
	if err != nil {
		s.logger.Error("begin tx failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to import")
		return
	}
	defer func() { _ = tx.Rollback() }()

	var upserted int64
	var deactivated int64

	for _, group := range importReq.Series {
		series, err := s.repo.GetSeriesByKey(r.Context(), tenantID, group.PartnerRoleID, group.ProductID, group.SeriesName)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				created, err := s.repo.UpsertSeries(r.Context(), tenantID, channelID, group.PartnerRoleID, group.ProductID, group.SeriesName, group.Mode, 1, true)
				if err != nil {
					s.logger.Error("ensure series failed", zap.Error(err))
					writeError(w, http.StatusInternalServerError, "failed to import")
					return
				}
				series = created
			} else {
				s.logger.Error("get series by key failed", zap.Error(err))
				writeError(w, http.StatusInternalServerError, "failed to import")
				return
			}
		}

		for contentVersion, items := range group.ItemsByVersion {
			keep := make([]int, 0, len(items))
			for _, item := range items {
				keep = append(keep, item.SeqNo)
				if _, err := s.repo.UpsertContentItemTx(r.Context(), tx, tenantID, seriesChannelID(series, channelID), series.ID, contentVersion, item.SeqNo, item.MessageText, item.IsActive); err != nil {
					s.logger.Error("upsert content item failed", zap.Error(err))
					writeError(w, http.StatusInternalServerError, "failed to import")
					return
				}
				upserted++
			}
			sort.Ints(keep)
			n, err := s.repo.DeactivateMissingContentItemsTx(r.Context(), tx, series.ID, contentVersion, keep)
			if err != nil {
				s.logger.Error("deactivate missing content failed", zap.Error(err))
				writeError(w, http.StatusInternalServerError, "failed to import")
				return
			}
			deactivated += n
		}
	}

	if err := tx.Commit(); err != nil {
		s.logger.Error("commit import failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to import")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"dry_run":      false,
		"series_count": len(importReq.Series),
		"row_count":    importReq.RowCount,
		"upserted":     upserted,
		"deactivated":  deactivated,
	})
}

func (s *Server) tenantScope(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	identity, _ := tenantctx.FromContext(r.Context())
	tenantID := strings.TrimSpace(identity.TenantID)
	if tenantID == "" && identity.PlatformScoped {
		tenantID = firstNonBlank(r.Header.Get(tenantctx.HeaderTenantID), r.URL.Query().Get("tenantId"), r.URL.Query().Get("tenant_id"))
	}
	if tenantID == "" {
		writeError(w, http.StatusForbidden, "tenant context required")
		return "", "", false
	}
	channelID := firstNonBlank(
		r.Header.Get("X-Tenant-Channel-Id"),
		r.Header.Get("X-Channel-Id"),
		r.URL.Query().Get("channelId"),
		r.URL.Query().Get("channel_id"),
	)
	return tenantID, channelID, true
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func seriesMatchesChannel(series *domain.MessageSeries, channelID string) bool {
	channelID = strings.TrimSpace(channelID)
	return channelID == "" || (series != nil && series.ChannelID != nil && strings.TrimSpace(*series.ChannelID) == channelID)
}

func seriesChannelID(series *domain.MessageSeries, fallback string) string {
	if series != nil && series.ChannelID != nil && strings.TrimSpace(*series.ChannelID) != "" {
		return strings.TrimSpace(*series.ChannelID)
	}
	return strings.TrimSpace(fallback)
}

func splitPath(p string) []string {
	// trim leading/trailing slashes and split
	start := 0
	for start < len(p) && p[start] == '/' {
		start++
	}
	end := len(p)
	for end > start && p[end-1] == '/' {
		end--
	}
	if end <= start {
		return nil
	}
	p = p[start:end]
	var parts []string
	cur := 0
	for i := 0; i <= len(p); i++ {
		if i == len(p) || p[i] == '/' {
			if i > cur {
				parts = append(parts, p[cur:i])
			}
			cur = i + 1
		}
	}
	return parts
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "y":
		return true, nil
	case "0", "false", "no", "n":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool")
	}
}

func parseClock(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty")
	}
	layouts := []string{"15:04", "15:04:05"}
	var t time.Time
	var err error
	for _, layout := range layouts {
		t, err = time.Parse(layout, s)
		if err == nil {
			return time.Date(2000, 1, 1, t.Hour(), t.Minute(), t.Second(), 0, time.UTC), nil
		}
	}
	return time.Time{}, err
}

type csvImportRequest struct {
	RowCount int
	Series   []csvSeriesGroup
}

type csvSeriesGroup struct {
	PartnerRoleID  int
	ProductID      int
	SeriesName     string
	Mode           string
	ItemsByVersion map[int][]csvItem
}

type csvItem struct {
	ContentVersion int
	SeqNo          int
	MessageText    string
	IsActive       bool
}

func parseCSVImport(r io.Reader) (*csvImportRequest, []map[string]any) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, []map[string]any{{"line": 1, "error": "missing header"}}
	}

	col := map[string]int{}
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}

	required := []string{"partner_role_id", "product_id", "series_name", "mode", "content_version", "message_text", "is_active"}
	for _, k := range required {
		if _, ok := col[k]; !ok {
			return nil, []map[string]any{{"line": 1, "error": fmt.Sprintf("missing required column: %s", k)}}
		}
	}
	// Optional
	seqNoIdx, hasSeq := col["seq_no"]

	type key struct {
		pr  int
		pid int
		n   string
	}

	seriesMap := map[key]*csvSeriesGroup{}
	seriesOrder := make([]key, 0)
	nextSeq := map[key]map[int]int{} // key -> contentVersion -> nextSeq

	var errs []map[string]any
	line := 1
	rows := 0

	for {
		line++
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errs = append(errs, map[string]any{"line": line, "error": "invalid csv row"})
			continue
		}
		rows++

		get := func(name string) string {
			idx, ok := col[name]
			if !ok || idx < 0 || idx >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[idx])
		}

		pr, err := strconv.Atoi(get("partner_role_id"))
		if err != nil || pr <= 0 {
			errs = append(errs, map[string]any{"line": line, "error": "invalid partner_role_id"})
			continue
		}
		pid, err := strconv.Atoi(get("product_id"))
		if err != nil || pid <= 0 {
			errs = append(errs, map[string]any{"line": line, "error": "invalid product_id"})
			continue
		}
		name := get("series_name")
		if name == "" {
			errs = append(errs, map[string]any{"line": line, "error": "missing series_name"})
			continue
		}
		mode := strings.ToUpper(get("mode"))
		if mode == "" {
			mode = "SEQUENTIAL"
		}
		if mode != "SEQUENTIAL" && mode != "POOL" {
			errs = append(errs, map[string]any{"line": line, "error": "invalid mode"})
			continue
		}

		cv, err := strconv.Atoi(get("content_version"))
		if err != nil || cv <= 0 {
			errs = append(errs, map[string]any{"line": line, "error": "invalid content_version"})
			continue
		}

		msg := get("message_text")
		if msg == "" {
			errs = append(errs, map[string]any{"line": line, "error": "missing message_text"})
			continue
		}
		if len(msg) > 2000 {
			errs = append(errs, map[string]any{"line": line, "error": "message_text too long"})
			continue
		}

		active, err := parseBool(get("is_active"))
		if err != nil {
			errs = append(errs, map[string]any{"line": line, "error": "invalid is_active"})
			continue
		}

		k := key{pr: pr, pid: pid, n: name}
		g, ok := seriesMap[k]
		if !ok {
			g = &csvSeriesGroup{
				PartnerRoleID:  pr,
				ProductID:      pid,
				SeriesName:     name,
				Mode:           mode,
				ItemsByVersion: map[int][]csvItem{},
			}
			seriesMap[k] = g
			seriesOrder = append(seriesOrder, k)
			nextSeq[k] = map[int]int{}
		} else if g.Mode != mode {
			errs = append(errs, map[string]any{"line": line, "error": "conflicting mode for series"})
			continue
		}

		seqNo := 0
		if hasSeq {
			raw := strings.TrimSpace(rec[seqNoIdx])
			if raw != "" {
				n, err := strconv.Atoi(raw)
				if err != nil || n <= 0 {
					errs = append(errs, map[string]any{"line": line, "error": "invalid seq_no"})
					continue
				}
				seqNo = n
			}
		}

		if seqNo == 0 {
			// For POOL we allow blank seq_no; generate a stable seq_no within this import file.
			if nextSeq[k][cv] == 0 {
				nextSeq[k][cv] = 1
			}
			seqNo = nextSeq[k][cv]
			nextSeq[k][cv]++
		} else {
			if nextSeq[k][cv] <= seqNo {
				nextSeq[k][cv] = seqNo + 1
			}
		}

		g.ItemsByVersion[cv] = append(g.ItemsByVersion[cv], csvItem{
			ContentVersion: cv,
			SeqNo:          seqNo,
			MessageText:    msg,
			IsActive:       active,
		})
	}

	if len(errs) > 0 {
		return nil, errs
	}

	out := &csvImportRequest{RowCount: rows}
	out.Series = make([]csvSeriesGroup, 0, len(seriesOrder))
	for _, k := range seriesOrder {
		out.Series = append(out.Series, *seriesMap[k])
	}
	return out, nil
}
