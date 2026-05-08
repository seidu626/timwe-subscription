package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
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
	channelKeyRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,118}[a-z0-9]$`)
)

var allowedChannelCapabilities = map[string]struct{}{
	"optin":   {},
	"confirm": {},
	"mt":      {},
	"charge":  {},
}

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

func (s *AdminManagementService) CreateProduct(tenantID string, input *domain.AdminProduct, actor, requestID *string) (*domain.AdminProduct, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrTenantContextMissing
	}
	if err := validateProductInput(input); err != nil {
		return nil, err
	}
	input.TenantID = tenantID
	created, err := s.repo.CreateProduct(input)
	if err != nil {
		if errors.Is(err, repository.ErrAdminConflict) {
			return nil, ErrAdminConflict
		}
		return nil, err
	}
	s.logActivity(tenantID, "product", fmt.Sprintf("%d", created.ID), "create", actor, requestID, nil, created, map[string]any{
		"product_id": created.ProductID,
	})
	return created, nil
}

func (s *AdminManagementService) UpdateProduct(tenantID string, id int, input *domain.AdminProduct, actor, requestID *string) (*domain.AdminProduct, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrTenantContextMissing
	}
	if id <= 0 {
		return nil, fmt.Errorf("%w: product id is required", ErrInvalidInput)
	}
	if err := validateProductInput(input); err != nil {
		return nil, err
	}

	before, err := s.repo.GetProductByID(tenantID, id)
	if err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return nil, ErrAdminNotFound
		}
		return nil, err
	}

	input.TenantID = tenantID
	updated, err := s.repo.UpdateProduct(tenantID, id, input)
	if err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return nil, ErrAdminNotFound
		}
		if errors.Is(err, repository.ErrAdminConflict) {
			return nil, ErrAdminConflict
		}
		return nil, err
	}

	s.logActivity(tenantID, "product", fmt.Sprintf("%d", updated.ID), "update", actor, requestID, before, updated, map[string]any{
		"product_id": updated.ProductID,
	})
	return updated, nil
}

func (s *AdminManagementService) DeleteProduct(tenantID string, id int, actor, requestID *string) error {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return ErrTenantContextMissing
	}
	if id <= 0 {
		return fmt.Errorf("%w: product id is required", ErrInvalidInput)
	}

	before, err := s.repo.GetProductByID(tenantID, id)
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

	if err := s.repo.DeleteProduct(tenantID, id); err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return ErrAdminNotFound
		}
		return err
	}

	s.logActivity(tenantID, "product", fmt.Sprintf("%d", before.ID), "delete", actor, requestID, before, nil, map[string]any{
		"product_id": before.ProductID,
	})
	return nil
}

func (s *AdminManagementService) BatchUpsertProducts(tenantID string, items []*domain.AdminProduct, actor, requestID *string) (int, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return 0, ErrTenantContextMissing
	}
	if len(items) == 0 {
		return 0, fmt.Errorf("%w: products list is empty", ErrInvalidInput)
	}
	for i := range items {
		if err := validateProductInput(items[i]); err != nil {
			return 0, fmt.Errorf("%w: item %d: %v", ErrInvalidInput, i+1, err)
		}
		items[i].TenantID = tenantID
	}

	n, err := s.repo.BatchUpsertProducts(tenantID, items)
	if err != nil {
		return 0, err
	}

	s.logActivity(tenantID, "product", "batch", "batch_upsert", actor, requestID, nil, nil, map[string]any{
		"count": n,
	})
	return n, nil
}

func (s *AdminManagementService) ListChannels(filter *domain.ChannelListFilter) ([]*domain.AdminChannel, int, error) {
	if filter == nil {
		filter = &domain.ChannelListFilter{Limit: 20}
	}
	filter.Provider = strings.ToLower(strings.TrimSpace(filter.Provider))
	filter.Country = strings.ToUpper(strings.TrimSpace(filter.Country))
	return s.repo.ListChannels(filter)
}

func (s *AdminManagementService) CreateChannel(tenantID string, input *domain.ChannelCreateInput, actor, requestID *string) (*domain.AdminChannel, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrTenantContextMissing
	}
	normalized, err := normalizeChannelCreateInput(input)
	if err != nil {
		return nil, err
	}

	status := domain.ChannelStatusActive
	if normalized.Enabled != nil && !*normalized.Enabled {
		status = domain.ChannelStatusInactive
	}
	channel := &domain.AdminChannel{
		TenantID:     tenantID,
		ChannelKey:   normalized.ChannelKey,
		Provider:     normalized.Provider,
		Country:      normalized.Country,
		Operator:     normalized.Operator,
		Capabilities: normalized.Capabilities,
		Status:       status,
		Enabled:      status == domain.ChannelStatusActive,
	}
	entry := &domain.AdminActivityLog{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		Action:    "create",
		Actor:     actor,
		RequestID: requestID,
		AfterJSON: mustJSON(channel),
		Metadata: mustJSON(map[string]any{
			"channel_key":  channel.ChannelKey,
			"provider":     channel.Provider,
			"country":      channel.Country,
			"capabilities": channel.Capabilities,
		}),
		CreatedAt: time.Now().UTC(),
	}

	created, err := s.repo.CreateChannelWithActivityLog(channel, entry)
	if err != nil {
		if errors.Is(err, repository.ErrAdminConflict) {
			return nil, ErrAdminConflict
		}
		return nil, err
	}
	return created, nil
}

func (s *AdminManagementService) SetChannelEnabled(tenantID, channelID string, enabled bool, actor, requestID *string) (*domain.AdminChannel, error) {
	tenantID = strings.TrimSpace(tenantID)
	channelID = strings.TrimSpace(channelID)
	if tenantID == "" {
		return nil, ErrTenantContextMissing
	}
	if channelID == "" {
		return nil, fmt.Errorf("%w: channel id is required", ErrInvalidInput)
	}
	status := domain.ChannelStatusInactive
	if enabled {
		status = domain.ChannelStatusActive
	}
	entry := &domain.AdminActivityLog{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		Action:    "set_enabled",
		Actor:     actor,
		RequestID: requestID,
		AfterJSON: mustJSON(map[string]any{
			"enabled": enabled,
			"status":  status,
		}),
		Metadata: mustJSON(map[string]any{
			"enabled": enabled,
		}),
		CreatedAt: time.Now().UTC(),
	}
	channel, err := s.repo.SetChannelStatusWithActivityLog(tenantID, channelID, status, entry)
	if err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return nil, ErrAdminNotFound
		}
		return nil, err
	}
	return channel, nil
}

func (s *AdminManagementService) ListUserbase(filter *domain.UserbaseListFilter) ([]*domain.UserbaseRecord, int, error) {
	return s.repo.ListUserbase(filter)
}

func (s *AdminManagementService) UpsertUserbase(tenantID, msisdn, userType string, actor, requestID *string) (*domain.UserbaseRecord, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrTenantContextMissing
	}
	normalizedMSISDN, normalizedType, err := normalizeUserbaseInput(msisdn, userType)
	if err != nil {
		return nil, err
	}

	var before *domain.UserbaseRecord
	before, _ = s.repo.GetUserbaseByMSISDN(tenantID, normalizedMSISDN)

	updated, err := s.repo.UpsertUserbase(tenantID, normalizedMSISDN, normalizedType)
	if err != nil {
		return nil, err
	}

	action := "create"
	if before != nil {
		action = "update"
	}
	s.logActivity(tenantID, "userbase", updated.MSISDN, action, actor, requestID, before, updated, nil)
	return updated, nil
}

func (s *AdminManagementService) DeleteUserbase(tenantID, msisdn string, actor, requestID *string) error {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return ErrTenantContextMissing
	}
	normalizedMSISDN, _, err := normalizeUserbaseInput(msisdn, "BLACKLISTED")
	if err != nil {
		return err
	}

	before, err := s.repo.GetUserbaseByMSISDN(tenantID, normalizedMSISDN)
	if err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return ErrAdminNotFound
		}
		return err
	}

	if err := s.repo.DeleteUserbase(tenantID, normalizedMSISDN); err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return ErrAdminNotFound
		}
		return err
	}

	s.logActivity(tenantID, "userbase", normalizedMSISDN, "delete", actor, requestID, before, nil, nil)
	return nil
}

func (s *AdminManagementService) ImportUserbase(tenantID, filename string, rows []domain.UserbaseImportInputRow, actor, requestID *string) (*domain.UserbaseImportJob, []*domain.UserbaseImportError, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, nil, ErrTenantContextMissing
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return nil, nil, fmt.Errorf("%w: filename is required", ErrInvalidInput)
	}

	job, err := s.repo.CreateUserbaseImportJob(tenantID, filename, actor)
	if err != nil {
		return nil, nil, err
	}

	errorsOut := make([]*domain.UserbaseImportError, 0)
	successCount := 0
	for _, row := range rows {
		msisdn, rowType, validationErr := normalizeUserbaseInput(row.MSISDN, row.Type)
		if validationErr != nil {
			errorsOut = append(errorsOut, &domain.UserbaseImportError{
				TenantID:     tenantID,
				JobID:        job.ID,
				RowNumber:    row.RowNumber,
				RawRow:       row.RawRow,
				ErrorMessage: validationErr.Error(),
			})
			continue
		}

		if _, err := s.repo.UpsertUserbase(tenantID, msisdn, rowType); err != nil {
			errorsOut = append(errorsOut, &domain.UserbaseImportError{
				TenantID:     tenantID,
				JobID:        job.ID,
				RowNumber:    row.RowNumber,
				RawRow:       row.RawRow,
				ErrorMessage: err.Error(),
			})
			continue
		}
		successCount++
	}

	if err := s.repo.InsertUserbaseImportErrors(tenantID, job.ID, errorsOut); err != nil {
		_ = s.repo.CompleteUserbaseImportJob(tenantID, job.ID, domain.UserbaseImportStatusFailed, len(rows), successCount, len(errorsOut))
		return nil, nil, err
	}

	status := domain.UserbaseImportStatusCompleted
	if successCount == 0 && len(rows) > 0 {
		status = domain.UserbaseImportStatusFailed
	}
	if err := s.repo.CompleteUserbaseImportJob(tenantID, job.ID, status, len(rows), successCount, len(errorsOut)); err != nil {
		return nil, nil, err
	}

	job.Status = status
	job.TotalRows = len(rows)
	job.SuccessRows = successCount
	job.FailedRows = len(errorsOut)
	now := time.Now().UTC()
	job.CompletedAt = &now

	s.logActivity(tenantID, "userbase_import", job.ID, "import", actor, requestID, nil, nil, map[string]any{
		"filename":     filename,
		"total_rows":   len(rows),
		"success_rows": successCount,
		"failed_rows":  len(errorsOut),
		"status":       status,
	})
	return job, errorsOut, nil
}

func (s *AdminManagementService) ListUserbaseImportJobs(tenantID string, page, pageSize int) ([]*domain.UserbaseImportJob, int, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, 0, ErrTenantContextMissing
	}
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
	return s.repo.ListUserbaseImportJobs(tenantID, pageSize, offset)
}

func (s *AdminManagementService) GetUserbaseImportJob(tenantID, jobID string) (*domain.UserbaseImportJob, []*domain.UserbaseImportError, int, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, nil, 0, ErrTenantContextMissing
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return nil, nil, 0, fmt.Errorf("%w: job id is required", ErrInvalidInput)
	}

	job, err := s.repo.GetUserbaseImportJob(tenantID, jobID)
	if err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return nil, nil, 0, ErrAdminNotFound
		}
		return nil, nil, 0, err
	}

	errorsOut, total, err := s.repo.ListUserbaseImportErrors(tenantID, jobID, 500, 0)
	if err != nil {
		return nil, nil, 0, err
	}
	return job, errorsOut, total, nil
}

func (s *AdminManagementService) ListActivityLogs(filter *domain.AdminActivityLogFilter) ([]*domain.AdminActivityLog, int, error) {
	return s.repo.ListActivityLogs(filter)
}

func (s *AdminManagementService) logActivity(
	tenantID string,
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
		TenantID:   strings.TrimSpace(tenantID),
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

func normalizeChannelCreateInput(input *domain.ChannelCreateInput) (*domain.ChannelCreateInput, error) {
	if input == nil {
		return nil, fmt.Errorf("%w: channel payload is required", ErrInvalidInput)
	}
	out := *input
	out.Provider = strings.ToLower(strings.TrimSpace(out.Provider))
	out.Country = strings.ToUpper(strings.TrimSpace(out.Country))
	if out.Operator != nil {
		operator := strings.TrimSpace(*out.Operator)
		if operator == "" {
			out.Operator = nil
		} else {
			out.Operator = &operator
		}
	}
	if out.Provider == "" {
		return nil, fmt.Errorf("%w: provider is required", ErrInvalidInput)
	}
	if !tenantCountryRe.MatchString(out.Country) {
		return nil, fmt.Errorf("%w: country must be an ISO 3166-1 alpha-2 code", ErrInvalidInput)
	}

	capabilities, err := normalizeChannelCapabilities(out.Capabilities)
	if err != nil {
		return nil, err
	}
	out.Capabilities = capabilities

	derivedKey := deriveChannelKey(out.Provider, out.Country, out.Operator)
	if provided := strings.ToLower(strings.TrimSpace(out.ChannelKey)); provided != "" && provided != derivedKey {
		return nil, fmt.Errorf("%w: channel_key is server-derived; use %s", ErrInvalidInput, derivedKey)
	}
	out.ChannelKey = derivedKey
	if !channelKeyRe.MatchString(out.ChannelKey) {
		return nil, fmt.Errorf("%w: channel_key is invalid", ErrInvalidInput)
	}
	return &out, nil
}

func normalizeChannelCapabilities(raw []string) ([]string, error) {
	seen := make(map[string]struct{}, len(raw))
	for _, value := range raw {
		capability := strings.ToLower(strings.TrimSpace(value))
		if capability == "" {
			continue
		}
		if _, ok := allowedChannelCapabilities[capability]; !ok {
			return nil, fmt.Errorf("%w: invalid_capability %s", ErrInvalidInput, capability)
		}
		seen[capability] = struct{}{}
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("%w: capabilities are required", ErrInvalidInput)
	}
	if _, hasCharge := seen["charge"]; hasCharge {
		if _, hasMT := seen["mt"]; !hasMT {
			return nil, fmt.Errorf("%w: invalid_capability charge requires mt", ErrInvalidInput)
		}
	}
	out := make([]string, 0, len(seen))
	for capability := range seen {
		out = append(out, capability)
	}
	sort.Strings(out)
	return out, nil
}

func deriveChannelKey(provider, country string, operator *string) string {
	parts := []string{
		normalizeKeyPart(provider),
		strings.ToLower(strings.TrimSpace(country)),
	}
	if operator != nil && strings.TrimSpace(*operator) != "" {
		parts = append(parts, normalizeKeyPart(*operator))
	}
	return strings.Join(parts, "-")
}

func normalizeKeyPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
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
