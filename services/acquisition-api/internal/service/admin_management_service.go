package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"go.uber.org/zap"
)

var (
	// ErrInvalidInput indicates an invalid request payload.
	ErrInvalidInput = errors.New("invalid input")
	// ErrAdminNotFound indicates an admin-managed resource was not found.
	ErrAdminNotFound = errors.New("admin resource not found")
	// ErrAdminConflict indicates an admin-managed resource conflicts with an existing resource.
	ErrAdminConflict = errors.New("admin resource conflict")
	// ErrAdminForbidden indicates the authenticated admin cannot perform the action.
	ErrAdminForbidden = errors.New("admin action forbidden")
	// ErrTenantContextMissing indicates a protected request has no accepted tenant context.
	ErrTenantContextMissing = errors.New("tenant context missing")
	// ErrTenantUnavailable hides inactive and unknown tenant details from tenant-scoped callers.
	ErrTenantUnavailable = errors.New("tenant unavailable")
)

var (
	tenantKeyRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,98}[a-z0-9]$`)
	tenantCountryRe = regexp.MustCompile(`^[A-Z]{2}$`)
)

// ProductDependencyError indicates a product cannot be deleted because it is still referenced.
type ProductDependencyError struct {
	Counts *domain.ProductDependencyCounts
}

func (e *ProductDependencyError) Error() string {
	return "product is referenced by active entities"
}

// AdminManagementService orchestrates admin management operations.
type AdminManagementService struct {
	repo   *repository.AdminManagementRepository
	logger *zap.Logger
}

func NewAdminManagementService(repo *repository.AdminManagementRepository, logger *zap.Logger) *AdminManagementService {
	return &AdminManagementService{repo: repo, logger: logger}
}

func (s *AdminManagementService) CreateTenant(input *domain.TenantCreateInput, identity tenantctx.Identity, actor, requestID *string) (*domain.AdminTenant, string, error) {
	if !identity.PlatformScoped {
		return nil, "", ErrAdminForbidden
	}
	if err := validateTenantCreateInput(input); err != nil {
		return nil, "", err
	}

	auditID := uuid.NewString()
	entry := &domain.AdminActivityLog{
		ID:        auditID,
		Action:    "create",
		Actor:     actor,
		RequestID: requestID,
		AfterJSON: mustJSON(input),
		Metadata: mustJSON(map[string]any{
			"tenant_key": input.TenantKey,
		}),
		CreatedAt: time.Now().UTC(),
	}

	tenant, err := s.repo.CreateTenantWithActivityLog(input, entry)
	if err != nil {
		if errors.Is(err, repository.ErrAdminConflict) {
			return nil, "", ErrAdminConflict
		}
		return nil, "", err
	}
	return tenant, auditID, nil
}

func (s *AdminManagementService) ResolveCurrentTenant(identity tenantctx.Identity) (*domain.AdminTenant, error) {
	if !identity.HasTenant() {
		return nil, ErrTenantContextMissing
	}

	var (
		tenant *domain.AdminTenant
		err    error
	)
	if id := strings.TrimSpace(identity.TenantID); id != "" {
		tenant, err = s.repo.GetTenantByID(id)
	} else {
		key := normalizeTenantKey(identity.TenantKey)
		if key == "" {
			return nil, ErrTenantContextMissing
		}
		tenant, err = s.repo.GetTenantByKey(key)
	}
	if err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return nil, ErrTenantUnavailable
		}
		return nil, err
	}
	if tenant.Status != domain.TenantStatusActive {
		return nil, ErrTenantUnavailable
	}
	return tenant, nil
}

func (s *AdminManagementService) ListProducts(filter *domain.ProductListFilter) ([]*domain.AdminProduct, int, error) {
	return s.repo.ListProducts(filter)
}

func (s *AdminManagementService) CreateProduct(input *domain.AdminProduct, actor, requestID *string) (*domain.AdminProduct, error) {
	if err := validateProductInput(input); err != nil {
		return nil, err
	}
	created, err := s.repo.CreateProduct(input)
	if err != nil {
		return nil, err
	}
	s.logActivity("product", fmt.Sprintf("%d", created.ID), "create", actor, requestID, nil, created, map[string]any{
		"product_id": created.ProductID,
	})
	return created, nil
}

func (s *AdminManagementService) UpdateProduct(id int, input *domain.AdminProduct, actor, requestID *string) (*domain.AdminProduct, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: product id is required", ErrInvalidInput)
	}
	if err := validateProductInput(input); err != nil {
		return nil, err
	}

	before, err := s.repo.GetProductByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return nil, ErrAdminNotFound
		}
		return nil, err
	}

	updated, err := s.repo.UpdateProduct(id, input)
	if err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return nil, ErrAdminNotFound
		}
		return nil, err
	}

	s.logActivity("product", fmt.Sprintf("%d", updated.ID), "update", actor, requestID, before, updated, map[string]any{
		"product_id": updated.ProductID,
	})
	return updated, nil
}

func (s *AdminManagementService) DeleteProduct(id int, actor, requestID *string) error {
	if id <= 0 {
		return fmt.Errorf("%w: product id is required", ErrInvalidInput)
	}

	before, err := s.repo.GetProductByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return ErrAdminNotFound
		}
		return err
	}

	counts, err := s.repo.CountProductDependencies(before.ProductID)
	if err != nil {
		return err
	}
	if counts.CampaignCount > 0 || counts.SubscriptionCount > 0 {
		return &ProductDependencyError{Counts: counts}
	}

	if err := s.repo.DeleteProduct(id); err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return ErrAdminNotFound
		}
		return err
	}

	s.logActivity("product", fmt.Sprintf("%d", before.ID), "delete", actor, requestID, before, nil, map[string]any{
		"product_id": before.ProductID,
	})
	return nil
}

func (s *AdminManagementService) BatchUpsertProducts(items []*domain.AdminProduct, actor, requestID *string) (int, error) {
	if len(items) == 0 {
		return 0, fmt.Errorf("%w: products list is empty", ErrInvalidInput)
	}
	for i := range items {
		if err := validateProductInput(items[i]); err != nil {
			return 0, fmt.Errorf("%w: item %d: %v", ErrInvalidInput, i+1, err)
		}
	}

	n, err := s.repo.BatchUpsertProducts(items)
	if err != nil {
		return 0, err
	}

	s.logActivity("product", "batch", "batch_upsert", actor, requestID, nil, nil, map[string]any{
		"count": n,
	})
	return n, nil
}

func (s *AdminManagementService) ListUserbase(filter *domain.UserbaseListFilter) ([]*domain.UserbaseRecord, int, error) {
	return s.repo.ListUserbase(filter)
}

func (s *AdminManagementService) UpsertUserbase(msisdn, userType string, actor, requestID *string) (*domain.UserbaseRecord, error) {
	normalizedMSISDN, normalizedType, err := normalizeUserbaseInput(msisdn, userType)
	if err != nil {
		return nil, err
	}

	var before *domain.UserbaseRecord
	before, _ = s.repo.GetUserbaseByMSISDN(normalizedMSISDN)

	updated, err := s.repo.UpsertUserbase(normalizedMSISDN, normalizedType)
	if err != nil {
		return nil, err
	}

	action := "create"
	if before != nil {
		action = "update"
	}
	s.logActivity("userbase", updated.MSISDN, action, actor, requestID, before, updated, nil)
	return updated, nil
}

func (s *AdminManagementService) DeleteUserbase(msisdn string, actor, requestID *string) error {
	normalizedMSISDN, _, err := normalizeUserbaseInput(msisdn, "BLACKLISTED")
	if err != nil {
		return err
	}

	before, err := s.repo.GetUserbaseByMSISDN(normalizedMSISDN)
	if err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return ErrAdminNotFound
		}
		return err
	}

	if err := s.repo.DeleteUserbase(normalizedMSISDN); err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return ErrAdminNotFound
		}
		return err
	}

	s.logActivity("userbase", normalizedMSISDN, "delete", actor, requestID, before, nil, nil)
	return nil
}

func (s *AdminManagementService) ImportUserbase(filename string, rows []domain.UserbaseImportInputRow, actor, requestID *string) (*domain.UserbaseImportJob, []*domain.UserbaseImportError, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return nil, nil, fmt.Errorf("%w: filename is required", ErrInvalidInput)
	}

	job, err := s.repo.CreateUserbaseImportJob(filename, actor)
	if err != nil {
		return nil, nil, err
	}

	errorsOut := make([]*domain.UserbaseImportError, 0)
	successCount := 0
	for _, row := range rows {
		msisdn, rowType, validationErr := normalizeUserbaseInput(row.MSISDN, row.Type)
		if validationErr != nil {
			errorsOut = append(errorsOut, &domain.UserbaseImportError{
				JobID:        job.ID,
				RowNumber:    row.RowNumber,
				RawRow:       row.RawRow,
				ErrorMessage: validationErr.Error(),
			})
			continue
		}

		if _, err := s.repo.UpsertUserbase(msisdn, rowType); err != nil {
			errorsOut = append(errorsOut, &domain.UserbaseImportError{
				JobID:        job.ID,
				RowNumber:    row.RowNumber,
				RawRow:       row.RawRow,
				ErrorMessage: err.Error(),
			})
			continue
		}
		successCount++
	}

	if err := s.repo.InsertUserbaseImportErrors(job.ID, errorsOut); err != nil {
		_ = s.repo.CompleteUserbaseImportJob(job.ID, domain.UserbaseImportStatusFailed, len(rows), successCount, len(errorsOut))
		return nil, nil, err
	}

	status := domain.UserbaseImportStatusCompleted
	if successCount == 0 && len(rows) > 0 {
		status = domain.UserbaseImportStatusFailed
	}
	if err := s.repo.CompleteUserbaseImportJob(job.ID, status, len(rows), successCount, len(errorsOut)); err != nil {
		return nil, nil, err
	}

	job.Status = status
	job.TotalRows = len(rows)
	job.SuccessRows = successCount
	job.FailedRows = len(errorsOut)
	now := time.Now().UTC()
	job.CompletedAt = &now

	s.logActivity("userbase_import", job.ID, "import", actor, requestID, nil, nil, map[string]any{
		"filename":     filename,
		"total_rows":   len(rows),
		"success_rows": successCount,
		"failed_rows":  len(errorsOut),
		"status":       status,
	})
	return job, errorsOut, nil
}

func (s *AdminManagementService) ListUserbaseImportJobs(page, pageSize int) ([]*domain.UserbaseImportJob, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	offset := (page - 1) * pageSize
	return s.repo.ListUserbaseImportJobs(pageSize, offset)
}

func (s *AdminManagementService) GetUserbaseImportJob(jobID string) (*domain.UserbaseImportJob, []*domain.UserbaseImportError, int, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return nil, nil, 0, fmt.Errorf("%w: job id is required", ErrInvalidInput)
	}

	job, err := s.repo.GetUserbaseImportJob(jobID)
	if err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return nil, nil, 0, ErrAdminNotFound
		}
		return nil, nil, 0, err
	}

	errorsOut, total, err := s.repo.ListUserbaseImportErrors(jobID, 500, 0)
	if err != nil {
		return nil, nil, 0, err
	}
	return job, errorsOut, total, nil
}

func (s *AdminManagementService) ListActivityLogs(filter *domain.AdminActivityLogFilter) ([]*domain.AdminActivityLog, int, error) {
	return s.repo.ListActivityLogs(filter)
}

func (s *AdminManagementService) logActivity(
	entityType string,
	entityID string,
	action string,
	actor *string,
	requestID *string,
	before any,
	after any,
	metadata any,
) {
	entry := &domain.AdminActivityLog{
		ID:         uuid.NewString(),
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
		Actor:      actor,
		RequestID:  requestID,
		CreatedAt:  time.Now().UTC(),
	}

	if before != nil {
		if bytes, err := json.Marshal(before); err == nil {
			entry.BeforeJSON = bytes
		}
	}
	if after != nil {
		if bytes, err := json.Marshal(after); err == nil {
			entry.AfterJSON = bytes
		}
	}
	if metadata != nil {
		if bytes, err := json.Marshal(metadata); err == nil {
			entry.Metadata = bytes
		}
	}

	if err := s.repo.CreateActivityLog(entry); err != nil {
		s.logger.Warn("failed to write admin activity log",
			zap.String("entity_type", entityType),
			zap.String("entity_id", entityID),
			zap.String("action", action),
			zap.Error(err),
		)
	}
}

func validateProductInput(input *domain.AdminProduct) error {
	if input == nil {
		return fmt.Errorf("%w: product payload is required", ErrInvalidInput)
	}
	input.ProductID = strings.TrimSpace(input.ProductID)
	input.Name = strings.TrimSpace(input.Name)
	input.ShortCode = strings.TrimSpace(input.ShortCode)

	if input.ProductID == "" {
		return fmt.Errorf("%w: product_id is required", ErrInvalidInput)
	}
	if input.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	if input.PricePointID <= 0 {
		return fmt.Errorf("%w: price_point_id must be > 0", ErrInvalidInput)
	}
	if input.PricePointValue < 0 {
		return fmt.Errorf("%w: price_point_value must be >= 0", ErrInvalidInput)
	}
	if input.ShortCode == "" {
		return fmt.Errorf("%w: short_code is required", ErrInvalidInput)
	}
	return nil
}

func validateTenantCreateInput(input *domain.TenantCreateInput) error {
	if input == nil {
		return fmt.Errorf("%w: tenant payload is required", ErrInvalidInput)
	}
	input.TenantKey = normalizeTenantKey(input.TenantKey)
	input.Name = strings.TrimSpace(input.Name)
	input.DefaultCountry = strings.ToUpper(strings.TrimSpace(input.DefaultCountry))
	input.Status = domain.TenantStatus(strings.ToUpper(strings.TrimSpace(string(input.Status))))
	if input.Status == "" {
		input.Status = domain.TenantStatusActive
	}
	if len(input.Metadata) == 0 {
		input.Metadata = []byte("{}")
	}

	if !tenantKeyRe.MatchString(input.TenantKey) {
		return fmt.Errorf("%w: tenant_key must be 3-100 lowercase letters, numbers, hyphen, or underscore", ErrInvalidInput)
	}
	if input.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	if input.Status != domain.TenantStatusActive && input.Status != domain.TenantStatusInactive {
		return fmt.Errorf("%w: status must be ACTIVE or INACTIVE", ErrInvalidInput)
	}
	if !tenantCountryRe.MatchString(input.DefaultCountry) {
		return fmt.Errorf("%w: default_country must be an ISO 3166-1 alpha-2 code", ErrInvalidInput)
	}
	if len(input.Metadata) > 8192 {
		return fmt.Errorf("%w: metadata must be 8192 bytes or less", ErrInvalidInput)
	}
	var metadata map[string]any
	if err := json.Unmarshal(input.Metadata, &metadata); err != nil {
		return fmt.Errorf("%w: metadata must be a JSON object", ErrInvalidInput)
	}
	if metadata == nil {
		return fmt.Errorf("%w: metadata must be a JSON object", ErrInvalidInput)
	}
	return nil
}

func normalizeTenantKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func mustJSON(v any) json.RawMessage {
	bytes, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return bytes
}

func normalizeUserbaseInput(msisdn, userType string) (string, string, error) {
	normalizedMSISDN, err := normalizeMSISDNForCountry(strings.TrimSpace(msisdn), "GH")
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	normalizedType := strings.ToUpper(strings.TrimSpace(userType))
	if normalizedType == "" {
		return "", "", fmt.Errorf("%w: type is required", ErrInvalidInput)
	}
	if len(normalizedType) > 50 {
		return "", "", fmt.Errorf("%w: type is too long", ErrInvalidInput)
	}
	return normalizedMSISDN, normalizedType, nil
}
