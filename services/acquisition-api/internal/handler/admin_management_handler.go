package handler

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/service"
	"github.com/valyala/fasthttp"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
)

type AdminManagementHandler struct {
	service *service.AdminManagementService
	logger  *zap.Logger
}

func NewAdminManagementHandler(service *service.AdminManagementService, logger *zap.Logger) *AdminManagementHandler {
	return &AdminManagementHandler{service: service, logger: logger}
}

type listProductsResponse struct {
	Products   []*domain.AdminProduct `json:"products"`
	TotalCount int                    `json:"total_count"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"page_size"`
}

type productPayload struct {
	ProductID       string  `json:"product_id"`
	Name            string  `json:"name"`
	PricePointID    int     `json:"price_point_id"`
	PricePointValue float64 `json:"price_point_value"`
	ShortCode       string  `json:"short_code"`
	PerformedBy     string  `json:"performed_by,omitempty"`
}

type batchProductPayload struct {
	Products    []productPayload `json:"products"`
	PerformedBy string           `json:"performed_by,omitempty"`
}

func (h *AdminManagementHandler) ListProducts(ctx *fasthttp.RequestCtx) {
	page, pageSize := parsePageArgs(ctx, 20, 200)
	filter := &domain.ProductListFilter{
		Limit:     pageSize,
		Offset:    (page - 1) * pageSize,
		Query:     strings.TrimSpace(string(ctx.QueryArgs().Peek("q"))),
		ShortCode: strings.TrimSpace(string(ctx.QueryArgs().Peek("short_code"))),
	}

	products, total, err := h.service.ListProducts(filter)
	if err != nil {
		h.logger.Error("Failed to list products", zap.Error(err))
		ctx.Error("Failed to list products", fasthttp.StatusInternalServerError)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, listProductsResponse{
		Products:   products,
		TotalCount: total,
		Page:       page,
		PageSize:   pageSize,
	})
}

func (h *AdminManagementHandler) CreateProduct(ctx *fasthttp.RequestCtx) {
	var req productPayload
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	product := payloadToProduct(&req)
	actor := actorFromPayloadOrRequest(req.PerformedBy, ctx)
	requestID := requestIDFromHeader(ctx)
	created, err := h.service.CreateProduct(product, actor, requestID)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}

	writeJSON(ctx, fasthttp.StatusCreated, created)
}

func (h *AdminManagementHandler) UpdateProduct(ctx *fasthttp.RequestCtx) {
	id, err := parseProductIDFromPath(string(ctx.Path()))
	if err != nil {
		ctx.Error("Invalid product id", fasthttp.StatusBadRequest)
		return
	}

	var req productPayload
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	product := payloadToProduct(&req)
	actor := actorFromPayloadOrRequest(req.PerformedBy, ctx)
	requestID := requestIDFromHeader(ctx)
	updated, err := h.service.UpdateProduct(id, product, actor, requestID)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, updated)
}

func (h *AdminManagementHandler) DeleteProduct(ctx *fasthttp.RequestCtx) {
	id, err := parseProductIDFromPath(string(ctx.Path()))
	if err != nil {
		ctx.Error("Invalid product id", fasthttp.StatusBadRequest)
		return
	}

	actor := actorFromHeader(ctx)
	requestID := requestIDFromHeader(ctx)
	if err := h.service.DeleteProduct(id, actor, requestID); err != nil {
		var depErr *service.ProductDependencyError
		switch {
		case errors.Is(err, service.ErrAdminNotFound):
			ctx.Error("Product not found", fasthttp.StatusNotFound)
		case errors.As(err, &depErr):
			writeJSON(ctx, fasthttp.StatusConflict, map[string]any{
				"error":             "product is referenced and cannot be deleted",
				"dependency_counts": depErr.Counts,
			})
		case errors.Is(err, service.ErrInvalidInput):
			ctx.Error(err.Error(), fasthttp.StatusBadRequest)
		default:
			h.logger.Error("Failed to delete product", zap.Error(err))
			ctx.Error("Failed to delete product", fasthttp.StatusInternalServerError)
		}
		return
	}

	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *AdminManagementHandler) BatchUpsertProducts(ctx *fasthttp.RequestCtx) {
	var req batchProductPayload
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	items := make([]*domain.AdminProduct, 0, len(req.Products))
	for i := range req.Products {
		items = append(items, payloadToProduct(&req.Products[i]))
	}

	actor := actorFromPayloadOrRequest(req.PerformedBy, ctx)
	requestID := requestIDFromHeader(ctx)
	count, err := h.service.BatchUpsertProducts(items, actor, requestID)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"message": "Batch upsert completed",
		"count":   count,
	})
}

type listUserbaseResponse struct {
	Records    []*domain.UserbaseRecord `json:"records"`
	TotalCount int                      `json:"total_count"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"page_size"`
}

type upsertUserbaseRequest struct {
	MSISDN      string `json:"msisdn"`
	Type        string `json:"type"`
	PerformedBy string `json:"performed_by,omitempty"`
}

func (h *AdminManagementHandler) ListUserbase(ctx *fasthttp.RequestCtx) {
	page, pageSize := parsePageArgs(ctx, 20, 200)
	filter := &domain.UserbaseListFilter{
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
		MSISDN: strings.TrimSpace(string(ctx.QueryArgs().Peek("msisdn"))),
		Type:   strings.TrimSpace(string(ctx.QueryArgs().Peek("type"))),
	}

	records, total, err := h.service.ListUserbase(filter)
	if err != nil {
		h.logger.Error("Failed to list userbase", zap.Error(err))
		ctx.Error("Failed to list userbase", fasthttp.StatusInternalServerError)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, listUserbaseResponse{
		Records:    records,
		TotalCount: total,
		Page:       page,
		PageSize:   pageSize,
	})
}

func (h *AdminManagementHandler) UpsertUserbase(ctx *fasthttp.RequestCtx) {
	var req upsertUserbaseRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	actor := actorFromPayloadOrRequest(req.PerformedBy, ctx)
	requestID := requestIDFromHeader(ctx)
	record, err := h.service.UpsertUserbase(req.MSISDN, req.Type, actor, requestID)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, record)
}

func (h *AdminManagementHandler) DeleteUserbase(ctx *fasthttp.RequestCtx) {
	msisdn, err := parseMSISDNFromPath(string(ctx.Path()))
	if err != nil {
		ctx.Error("Invalid msisdn", fasthttp.StatusBadRequest)
		return
	}

	actor := actorFromHeader(ctx)
	requestID := requestIDFromHeader(ctx)
	if err := h.service.DeleteUserbase(msisdn, actor, requestID); err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *AdminManagementHandler) ImportUserbase(ctx *fasthttp.RequestCtx) {
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		ctx.Error("file is required", fasthttp.StatusBadRequest)
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		ctx.Error("failed to open uploaded file", fasthttp.StatusInternalServerError)
		return
	}
	defer func(file multipart.File) { _ = file.Close() }(file)

	rows, err := parseImportRows(fileHeader.Filename, file)
	if err != nil {
		ctx.Error(fmt.Sprintf("failed to parse import file: %v", err), fasthttp.StatusBadRequest)
		return
	}

	actor := actorFromHeader(ctx)
	requestID := requestIDFromHeader(ctx)
	job, importErrors, err := h.service.ImportUserbase(fileHeader.Filename, rows, actor, requestID)
	if err != nil {
		h.logger.Error("Failed to import userbase", zap.Error(err))
		h.handleServiceError(ctx, err)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"job":    job,
		"errors": importErrors,
	})
}

func (h *AdminManagementHandler) ListUserbaseImports(ctx *fasthttp.RequestCtx) {
	page, pageSize := parsePageArgs(ctx, 20, 200)
	jobs, total, err := h.service.ListUserbaseImportJobs(page, pageSize)
	if err != nil {
		h.logger.Error("Failed to list userbase imports", zap.Error(err))
		ctx.Error("Failed to list userbase imports", fasthttp.StatusInternalServerError)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"jobs":        jobs,
		"total_count": total,
		"page":        page,
		"page_size":   pageSize,
	})
}

func (h *AdminManagementHandler) GetUserbaseImport(ctx *fasthttp.RequestCtx) {
	jobID, err := parseImportIDFromPath(string(ctx.Path()))
	if err != nil {
		ctx.Error("Invalid import job id", fasthttp.StatusBadRequest)
		return
	}

	job, errorsOut, totalErrors, err := h.service.GetUserbaseImportJob(jobID)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"job":          job,
		"errors":       errorsOut,
		"total_errors": totalErrors,
	})
}

func (h *AdminManagementHandler) ListActivityLogs(ctx *fasthttp.RequestCtx) {
	page, pageSize := parsePageArgs(ctx, 20, 200)
	filter := &domain.AdminActivityLogFilter{
		Limit:      pageSize,
		Offset:     (page - 1) * pageSize,
		EntityType: strings.TrimSpace(string(ctx.QueryArgs().Peek("entity_type"))),
		Action:     strings.TrimSpace(string(ctx.QueryArgs().Peek("action"))),
		Actor:      strings.TrimSpace(string(ctx.QueryArgs().Peek("actor"))),
	}

	if fromRaw := strings.TrimSpace(string(ctx.QueryArgs().Peek("from"))); fromRaw != "" {
		if t, err := time.Parse("2006-01-02", fromRaw); err == nil {
			filter.From = &t
		}
	}
	if toRaw := strings.TrimSpace(string(ctx.QueryArgs().Peek("to"))); toRaw != "" {
		if t, err := time.Parse("2006-01-02", toRaw); err == nil {
			t = t.Add(24*time.Hour - time.Second)
			filter.To = &t
		}
	}

	items, total, err := h.service.ListActivityLogs(filter)
	if err != nil {
		h.logger.Error("Failed to list activity logs", zap.Error(err))
		ctx.Error("Failed to list activity logs", fasthttp.StatusInternalServerError)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"items":       items,
		"total_count": total,
		"page":        page,
		"page_size":   pageSize,
	})
}

func (h *AdminManagementHandler) handleServiceError(ctx *fasthttp.RequestCtx, err error) {
	var depErr *service.ProductDependencyError
	switch {
	case errors.Is(err, service.ErrAdminNotFound):
		ctx.Error("Resource not found", fasthttp.StatusNotFound)
	case errors.Is(err, service.ErrInvalidInput):
		ctx.Error(err.Error(), fasthttp.StatusBadRequest)
	case errors.As(err, &depErr):
		writeJSON(ctx, fasthttp.StatusConflict, map[string]any{
			"error":             depErr.Error(),
			"dependency_counts": depErr.Counts,
		})
	default:
		h.logger.Error("Unhandled admin management error", zap.Error(err))
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
	}
}

func payloadToProduct(req *productPayload) *domain.AdminProduct {
	return &domain.AdminProduct{
		ProductID:       req.ProductID,
		Name:            req.Name,
		PricePointID:    req.PricePointID,
		PricePointValue: req.PricePointValue,
		ShortCode:       req.ShortCode,
	}
}

func parsePageArgs(ctx *fasthttp.RequestCtx, defaultSize, maxSize int) (int, int) {
	page := 1
	pageSize := defaultSize
	if p := ctx.QueryArgs().GetUintOrZero("page"); p > 0 {
		page = int(p)
	}
	if ps := ctx.QueryArgs().GetUintOrZero("page_size"); ps > 0 {
		pageSize = int(ps)
	}
	if pageSize > maxSize {
		pageSize = maxSize
	}
	return page, pageSize
}

func parseProductIDFromPath(path string) (int, error) {
	parts := splitPathParts(path)
	if len(parts) < 4 {
		return 0, errors.New("invalid path")
	}
	return strconv.Atoi(parts[len(parts)-1])
}

func parseMSISDNFromPath(path string) (string, error) {
	parts := splitPathParts(path)
	if len(parts) < 4 {
		return "", errors.New("invalid path")
	}
	msisdn := strings.TrimSpace(parts[len(parts)-1])
	if msisdn == "" {
		return "", errors.New("missing msisdn")
	}
	return msisdn, nil
}

func parseImportIDFromPath(path string) (string, error) {
	parts := splitPathParts(path)
	if len(parts) < 5 {
		return "", errors.New("invalid path")
	}
	id := strings.TrimSpace(parts[len(parts)-1])
	if id == "" {
		return "", errors.New("missing import id")
	}
	return id, nil
}

func actorFromPayloadOrRequest(performedBy string, ctx *fasthttp.RequestCtx) *string {
	performedBy = strings.TrimSpace(performedBy)
	if performedBy != "" {
		return &performedBy
	}
	return actorFromHeader(ctx)
}

func actorFromHeader(ctx *fasthttp.RequestCtx) *string {
	if actor := strings.TrimSpace(string(ctx.Request.Header.Peek("X-Admin-User"))); actor != "" {
		return &actor
	}
	return nil
}

func requestIDFromHeader(ctx *fasthttp.RequestCtx) *string {
	if reqID := strings.TrimSpace(string(ctx.Request.Header.Peek("x-requestid"))); reqID != "" {
		return &reqID
	}
	return nil
}

func parseImportRows(filename string, r io.Reader) ([]domain.UserbaseImportInputRow, error) {
	lower := strings.ToLower(strings.TrimSpace(filename))
	switch {
	case strings.HasSuffix(lower, ".csv"):
		return parseCSVImportRows(r)
	case strings.HasSuffix(lower, ".xlsx"):
		return parseXLSXImportRows(r)
	default:
		return nil, fmt.Errorf("unsupported file format, use .csv or .xlsx")
	}
}

func parseCSVImportRows(r io.Reader) ([]domain.UserbaseImportInputRow, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	start := 0
	if len(rows[0]) >= 2 {
		h0 := strings.ToLower(strings.TrimSpace(rows[0][0]))
		h1 := strings.ToLower(strings.TrimSpace(rows[0][1]))
		if h0 == "msisdn" && h1 == "type" {
			start = 1
		}
	}

	out := make([]domain.UserbaseImportInputRow, 0, len(rows)-start)
	for i := start; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}
		msisdn, userType := "", ""
		if len(row) > 0 {
			msisdn = strings.TrimSpace(row[0])
		}
		if len(row) > 1 {
			userType = strings.TrimSpace(row[1])
		}
		raw := strings.Join(row, ",")
		out = append(out, domain.UserbaseImportInputRow{
			RowNumber: i + 1,
			MSISDN:    msisdn,
			Type:      userType,
			RawRow:    raw,
		})
	}
	return out, nil
}

func parseXLSXImportRows(r io.Reader) ([]domain.UserbaseImportInputRow, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, nil
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	start := 0
	if len(rows[0]) >= 2 {
		h0 := strings.ToLower(strings.TrimSpace(rows[0][0]))
		h1 := strings.ToLower(strings.TrimSpace(rows[0][1]))
		if h0 == "msisdn" && h1 == "type" {
			start = 1
		}
	}

	out := make([]domain.UserbaseImportInputRow, 0, len(rows)-start)
	for i := start; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}
		msisdn, userType := "", ""
		if len(row) > 0 {
			msisdn = strings.TrimSpace(row[0])
		}
		if len(row) > 1 {
			userType = strings.TrimSpace(row[1])
		}
		raw := strings.Join(row, ",")
		out = append(out, domain.UserbaseImportInputRow{
			RowNumber: i + 1,
			MSISDN:    msisdn,
			Type:      userType,
			RawRow:    raw,
		})
	}
	return out, nil
}
