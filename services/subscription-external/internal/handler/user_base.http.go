package handler

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"

	"github.com/valyala/fasthttp"
)

type UserBaseHandler struct {
	service *service.UserBaseService
	config  *config.Config
	logger  *zap.Logger
}

func NewUserBaseHandler(logger *zap.Logger, service *service.UserBaseService, c *config.Config) *UserBaseHandler {
	return &UserBaseHandler{
		logger:  logger,
		service: service, config: c}
}

// UploadHandler godoc
// @Summary Legacy user base upload endpoint (retired)
// @Description This endpoint is retired. Use the authenticated POST /v1/admin/userbase/imports endpoint.
// @Tags UserBase
// @Produce json
// @Failure 410 {object} map[string]string "Legacy endpoint retired"
// @Router /api/v1/userbase/upload [post]
func (h *UserBaseHandler) UploadHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusGone)
	ctx.SetBodyString(`{"error":"legacy userbase upload endpoint is retired","canonical_endpoint":"/v1/admin/userbase/imports","message":"Use the authenticated tenant-scoped userbase import endpoint."}`)
}

// parseCSV reads and parses CSV file content into UserBase records
func parseCSV(file io.Reader) ([]*domain.UserBase, error) {
	var records []*domain.UserBase
	r := csv.NewReader(file)
	r.TrimLeadingSpace = true
	r.ReuseRecord = true

	// Skip the header
	_, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("error reading CSV header: %v", err)
	}

	// Read each record
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading CSV row: %v", err)
		}
		// Populate UserBase record
		records = append(records, &domain.UserBase{
			Msisdn: strings.Trim(row[0], " "),
			Type:   strings.Trim(row[1], " "),
		})
	}
	return records, nil
}

// parseExcel reads and parses Excel file content into UserBase records
func parseExcel(file io.Reader) ([]*domain.UserBase, error) {
	var records []*domain.UserBase
	f, err := excelize.OpenReader(file)
	if err != nil {
		return nil, fmt.Errorf("error opening Excel file: %v", err)
	}

	// Assuming the data is in the first sheet
	rows, err := f.GetRows(f.GetSheetName(0))
	if err != nil {
		return nil, fmt.Errorf("error reading Excel rows: %v", err)
	}

	// Skip the header row
	for _, row := range rows[1:] {
		if len(row) < 2 {
			continue // skip if row doesn't have enough columns
		}
		// Populate UserBase record
		records = append(records, &domain.UserBase{
			Msisdn: strings.Trim(row[0], " "),
			Type:   strings.Trim(row[1], " "),
		})
	}
	return records, nil
}
