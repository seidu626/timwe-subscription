package handler

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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
// @Summary Upload user base file
// @Description Upload and process CSV or XLSX files containing user base data
// @Tags UserBase
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "CSV or XLSX file containing user data"
// @Success 200 {string} string "Successfully processed records"
// @Failure 400 {string} string "Invalid file format or failed to retrieve file"
// @Failure 500 {string} string "Failed to parse file or store records"
// @Router /api/v1/userbase/upload [post]
func (h *UserBaseHandler) UploadHandler(ctx *fasthttp.RequestCtx) {
	// Extract uploaded file
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		ctx.Error("Failed to retrieve file", fasthttp.StatusBadRequest)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		ctx.Error("Unable to open uploaded file", fasthttp.StatusInternalServerError)
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

	// Determine file type and process accordingly
	filename := fileHeader.Filename
	var userRecords []*domain.UserBase
	if strings.HasSuffix(filename, ".csv") {
		userRecords, err = parseCSV(file)
	} else if strings.HasSuffix(filename, ".xlsx") {
		userRecords, err = parseExcel(file)
	} else {
		ctx.Error("Unsupported file format. Upload CSV or XLSX files only", fasthttp.StatusBadRequest)
		return
	}

	if err != nil {
		ctx.Error(fmt.Sprintf("Failed to parse file: %v", err), fasthttp.StatusInternalServerError)
		return
	}

	// Process and store valid records in database
	err = h.service.ValidateAndStoreUserRecords(context.Background(), userRecords)
	if err != nil {
		ctx.Error(fmt.Sprintf("Failed to store records: %v", err), fasthttp.StatusInternalServerError)
		return
	}

	// Create proper JSON response
	response := domain.SubscribeResponse{
		Status:  "success",
		Message: "Successfully processed records",
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
		ctx.Error("Failed to format response", fasthttp.StatusInternalServerError)
		return
	}
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
