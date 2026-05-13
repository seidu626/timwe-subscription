package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
	// ErrAdminInvalidState indicates the target resource cannot accept the requested mutation now.
	ErrAdminInvalidState = errors.New("admin resource invalid state")
	// ErrAdminDependencyUnavailable indicates a required backend dependency is unavailable.
	ErrAdminDependencyUnavailable = errors.New("admin dependency unavailable")
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

const defaultChannelCredentialPurpose = "provider_api"

var channelCredentialPurposeRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,78}[a-z0-9]$`)

type ChannelCredentialSecretInput struct {
	TenantID    string
	ChannelID   string
	Purpose     string
	SecretValue string
}

type ChannelCredentialSecretRef struct {
	SecretRef        string
	SecretRefDisplay string
	FingerprintInput string
}

type ChannelCredentialSecretStore interface {
	PutChannelCredential(ctx context.Context, input ChannelCredentialSecretInput) (ChannelCredentialSecretRef, error)
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
	repo              *repository.AdminManagementRepository
	logger            *zap.Logger
	credentialSecrets ChannelCredentialSecretStore
}

func NewAdminManagementService(repo *repository.AdminManagementRepository, logger *zap.Logger) *AdminManagementService {
	return &AdminManagementService{repo: repo, logger: logger}
}

func (s *AdminManagementService) SetChannelCredentialSecretStore(store ChannelCredentialSecretStore) {
	s.credentialSecrets = store
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

func (s *AdminManagementService) ListTenants(identity tenantctx.Identity, filter *domain.TenantListFilter) ([]*domain.AdminTenant, int, error) {
	if !identity.PlatformScoped {
		return nil, 0, ErrAdminForbidden
	}
	if filter == nil {
		filter = &domain.TenantListFilter{Limit: 20}
	}
	filter.Status = domain.TenantStatus(strings.ToUpper(strings.TrimSpace(string(filter.Status))))
	if filter.Status != "" && filter.Status != domain.TenantStatusActive && filter.Status != domain.TenantStatusInactive {
		return nil, 0, fmt.Errorf("%w: status must be ACTIVE or INACTIVE", ErrInvalidInput)
	}
	filter.Query = strings.TrimSpace(filter.Query)
	return s.repo.ListTenants(filter)
}

func (s *AdminManagementService) UpdateTenant(id string, input *domain.TenantUpdateInput, identity tenantctx.Identity, actor, requestID *string) (*domain.AdminTenant, string, error) {
	if !identity.PlatformScoped {
		return nil, "", ErrAdminForbidden
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, "", fmt.Errorf("%w: tenant id is required", ErrInvalidInput)
	}
	if err := validateTenantUpdateInput(input); err != nil {
		return nil, "", err
	}

	before, err := s.repo.GetTenantByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return nil, "", ErrAdminNotFound
		}
		return nil, "", err
	}

	auditID := uuid.NewString()
	entry := &domain.AdminActivityLog{
		ID:         auditID,
		TenantID:   before.ID,
		Action:     "update",
		Actor:      actor,
		RequestID:  requestID,
		BeforeJSON: mustJSON(before),
		AfterJSON:  mustJSON(input),
		Metadata: mustJSON(map[string]any{
			"tenant_key": before.TenantKey,
		}),
		CreatedAt: time.Now().UTC(),
	}
	tenant, err := s.repo.UpdateTenantWithActivityLog(id, input, entry)
	if err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return nil, "", ErrAdminNotFound
		}
		return nil, "", err
	}
	entry.AfterJSON = mustJSON(tenant)
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

func (s *AdminManagementService) ListAuthorizedTenantWorkspaces(identity tenantctx.Identity) (*domain.AdminTenantWorkspace, error) {
	if identity.PlatformScoped {
		tenants, _, err := s.ListTenants(identity, &domain.TenantListFilter{
			Limit:  500,
			Status: domain.TenantStatusActive,
		})
		if err != nil {
			return nil, err
		}
		return &domain.AdminTenantWorkspace{
			PlatformScoped: true,
			Tenants:        tenants,
		}, nil
	}

	tenants, err := s.repo.ListActiveTenantsForMember(identity.Subject, identity.Email)
	if err != nil {
		return nil, err
	}
	if len(tenants) == 0 && identity.HasTenant() {
		tenant, err := s.ResolveCurrentTenant(identity)
		if err == nil {
			tenants = append(tenants, tenant)
		} else if !errors.Is(err, ErrTenantUnavailable) && !errors.Is(err, ErrTenantContextMissing) {
			return nil, err
		}
	}
	return &domain.AdminTenantWorkspace{
		PlatformScoped: false,
		Tenants:        tenants,
	}, nil
}

func (s *AdminManagementService) ListTenantMembers(tenantID string, identity tenantctx.Identity, filter *domain.TenantMemberListFilter) ([]*domain.AdminTenantMember, int, error) {
	if !identity.PlatformScoped {
		return nil, 0, ErrAdminForbidden
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, 0, fmt.Errorf("%w: tenant id is required", ErrInvalidInput)
	}
	if _, err := s.repo.GetTenantByID(tenantID); err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return nil, 0, ErrAdminNotFound
		}
		return nil, 0, err
	}
	if filter == nil {
		filter = &domain.TenantMemberListFilter{Limit: 20}
	}
	filter.TenantID = tenantID
	filter.Status = domain.TenantMemberStatus(strings.ToUpper(strings.TrimSpace(string(filter.Status))))
	if filter.Status != "" && filter.Status != domain.TenantMemberStatusActive && filter.Status != domain.TenantMemberStatusInactive {
		return nil, 0, fmt.Errorf("%w: status must be ACTIVE or INACTIVE", ErrInvalidInput)
	}
	filter.Query = strings.TrimSpace(filter.Query)
	return s.repo.ListTenantMembers(filter)
}

func (s *AdminManagementService) UpsertTenantMember(tenantID string, input *domain.TenantMemberInput, identity tenantctx.Identity, actor, requestID *string) (*domain.AdminTenantMember, string, error) {
	if !identity.PlatformScoped {
		return nil, "", ErrAdminForbidden
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, "", fmt.Errorf("%w: tenant id is required", ErrInvalidInput)
	}
	if _, err := s.repo.GetTenantByID(tenantID); err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return nil, "", ErrAdminNotFound
		}
		return nil, "", err
	}
	if input == nil {
		return nil, "", fmt.Errorf("%w: member payload is required", ErrInvalidInput)
	}
	input.TenantID = tenantID
	if err := validateTenantMemberInput(input); err != nil {
		return nil, "", err
	}

	auditID := uuid.NewString()
	entry := &domain.AdminActivityLog{
		ID:        auditID,
		Action:    "upsert",
		Actor:     actor,
		RequestID: requestID,
		AfterJSON: mustJSON(input),
		Metadata: mustJSON(map[string]any{
			"auth0_subject": input.Auth0Subject,
			"role":          input.Role,
			"status":        input.Status,
		}),
		CreatedAt: time.Now().UTC(),
	}
	member, err := s.repo.UpsertTenantMemberWithActivityLog(input, entry)
	if err != nil {
		return nil, "", err
	}
	return member, auditID, nil
}

func (s *AdminManagementService) DeactivateTenantMember(tenantID, auth0Subject string, identity tenantctx.Identity, actor, requestID *string) (string, error) {
	if !identity.PlatformScoped {
		return "", ErrAdminForbidden
	}
	tenantID = strings.TrimSpace(tenantID)
	auth0Subject = strings.TrimSpace(auth0Subject)
	if tenantID == "" || auth0Subject == "" {
		return "", fmt.Errorf("%w: tenant id and auth0 subject are required", ErrInvalidInput)
	}
	auditID := uuid.NewString()
	entry := &domain.AdminActivityLog{
		ID:        auditID,
		Action:    "deactivate",
		Actor:     actor,
		RequestID: requestID,
		Metadata: mustJSON(map[string]any{
			"auth0_subject": auth0Subject,
		}),
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.DeactivateTenantMemberWithActivityLog(tenantID, auth0Subject, entry); err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return "", ErrAdminNotFound
		}
		return "", err
	}
	return auditID, nil
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

func (s *AdminManagementService) ListChannelCredentials(filter *domain.ChannelCredentialListFilter) ([]*domain.AdminChannelCredential, int, error) {
	if filter == nil {
		filter = &domain.ChannelCredentialListFilter{Limit: 20}
	}
	filter.Purpose = normalizeCredentialPurpose(filter.Purpose)
	credentials, total, err := s.repo.ListChannelCredentials(filter)
	if err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return nil, 0, ErrAdminNotFound
		}
		return nil, 0, err
	}
	return credentials, total, nil
}

func (s *AdminManagementService) BindChannelCredential(ctx context.Context, tenantID, channelID string, input *domain.ChannelCredentialBindInput, actor, requestID *string) (*domain.AdminChannelCredential, error) {
	tenantID = strings.TrimSpace(tenantID)
	channelID = strings.TrimSpace(channelID)
	if tenantID == "" {
		return nil, ErrTenantContextMissing
	}
	if channelID == "" {
		return nil, fmt.Errorf("%w: channel id is required", ErrInvalidInput)
	}

	channel, err := s.repo.GetChannelByID(tenantID, channelID)
	if err != nil {
		if errors.Is(err, repository.ErrAdminNotFound) {
			return nil, ErrAdminNotFound
		}
		return nil, err
	}
	if !channel.IsRoutable() {
		return nil, fmt.Errorf("%w: channel_inactive", ErrAdminInvalidState)
	}
	normalized, err := s.normalizeChannelCredentialBindInput(ctx, tenantID, channelID, input)
	if err != nil {
		return nil, err
	}

	credential := &domain.AdminChannelCredential{
		TenantID:          tenantID,
		ChannelID:         channelID,
		Purpose:           normalized.Purpose,
		Status:            domain.ChannelCredentialStatusActive,
		SecretRef:         normalized.SecretRef,
		SecretRefDisplay:  normalized.SecretRefDisplay,
		SecretFingerprint: credentialFingerprint(tenantID, normalized.fingerprintInput()),
		CreatedBy:         actor,
	}
	entry := &domain.AdminActivityLog{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		Action:    "bind",
		Actor:     actor,
		RequestID: requestID,
		AfterJSON: mustJSON(map[string]any{
			"channel_id":       channelID,
			"purpose":          credential.Purpose,
			"redacted_display": credential.SecretRefDisplay,
			"status":           credential.Status,
		}),
		Metadata: mustJSON(map[string]any{
			"channel_id": channelID,
			"purpose":    credential.Purpose,
		}),
		CreatedAt: time.Now().UTC(),
	}

	created, err := s.repo.RotateChannelCredentialWithActivityLog(credential, entry)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrAdminNotFound):
			return nil, ErrAdminNotFound
		case errors.Is(err, repository.ErrAdminInvalidState):
			return nil, fmt.Errorf("%w: channel_inactive", ErrAdminInvalidState)
		case errors.Is(err, repository.ErrAdminConflict):
			return nil, ErrAdminConflict
		default:
			return nil, err
		}
	}
	return created, nil
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

type normalizedCredentialBindInput struct {
	Purpose          string
	SecretRef        string
	SecretRefDisplay string
	FingerprintInput string
}

func (n normalizedCredentialBindInput) fingerprintInput() string {
	if strings.TrimSpace(n.FingerprintInput) != "" {
		return n.FingerprintInput
	}
	return n.SecretRef
}

func (s *AdminManagementService) normalizeChannelCredentialBindInput(ctx context.Context, tenantID, channelID string, input *domain.ChannelCredentialBindInput) (*normalizedCredentialBindInput, error) {
	if input == nil {
		return nil, fmt.Errorf("%w: credential payload is required", ErrInvalidInput)
	}
	purpose := normalizeCredentialPurpose(input.Purpose)
	if !channelCredentialPurposeRe.MatchString(purpose) {
		return nil, fmt.Errorf("%w: purpose is invalid", ErrInvalidInput)
	}
	secretRef := strings.TrimSpace(input.SecretRef)
	secretValue := strings.TrimSpace(input.SecretValue)
	display := strings.TrimSpace(input.SecretRefDisplay)
	if secretRef != "" && secretValue != "" {
		return nil, fmt.Errorf("%w: provide secret_ref or secret_value, not both", ErrInvalidInput)
	}
	if secretRef == "" && secretValue == "" {
		return nil, fmt.Errorf("%w: secret_ref or secret_value is required", ErrInvalidInput)
	}

	if secretRef != "" {
		if err := validateSecretRef(secretRef); err != nil {
			return nil, err
		}
		display = redactSecretRef(secretRef)
		return &normalizedCredentialBindInput{
			Purpose:          purpose,
			SecretRef:        secretRef,
			SecretRefDisplay: display,
			FingerprintInput: secretRef,
		}, nil
	}

	if s.credentialSecrets == nil {
		return nil, fmt.Errorf("%w: secret_backend_unavailable", ErrAdminDependencyUnavailable)
	}
	ref, err := s.credentialSecrets.PutChannelCredential(ctx, ChannelCredentialSecretInput{
		TenantID:    tenantID,
		ChannelID:   channelID,
		Purpose:     purpose,
		SecretValue: secretValue,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: secret_backend_unavailable", ErrAdminDependencyUnavailable)
	}
	secretRef = strings.TrimSpace(ref.SecretRef)
	if err := validateSecretRef(secretRef); err != nil {
		return nil, err
	}
	display = redactSecretRef(secretRef)
	fingerprintInput := strings.TrimSpace(ref.FingerprintInput)
	if fingerprintInput == "" {
		fingerprintInput = secretValue
	}
	return &normalizedCredentialBindInput{
		Purpose:          purpose,
		SecretRef:        secretRef,
		SecretRefDisplay: display,
		FingerprintInput: fingerprintInput,
	}, nil
}

func normalizeCredentialPurpose(purpose string) string {
	purpose = strings.ToLower(strings.TrimSpace(purpose))
	if purpose == "" {
		return defaultChannelCredentialPurpose
	}
	return purpose
}

func validateSecretRef(secretRef string) error {
	for _, prefix := range []string{"vault://", "aws-sm://", "gcp-sm://", "azure-kv://", "secret://", "env://"} {
		if strings.HasPrefix(secretRef, prefix) && len(secretRef) > len(prefix) {
			return nil
		}
	}
	return fmt.Errorf("%w: secret_ref must use an allowed reference prefix", ErrInvalidInput)
}

func redactSecretRef(secretRef string) string {
	secretRef = strings.TrimSpace(secretRef)
	if secretRef == "" {
		return "[REDACTED]"
	}
	if idx := strings.Index(secretRef, "://"); idx > 0 {
		return secretRef[:idx+3] + "[REDACTED]"
	}
	return "[REDACTED]"
}

func credentialFingerprint(tenantID, value string) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(tenantID)))
	mac.Write([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(mac.Sum(nil))
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

func validateTenantUpdateInput(input *domain.TenantUpdateInput) error {
	if input == nil {
		return fmt.Errorf("%w: tenant payload is required", ErrInvalidInput)
	}
	hasChange := false
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return fmt.Errorf("%w: name is required", ErrInvalidInput)
		}
		input.Name = &name
		hasChange = true
	}
	if input.Status != nil {
		status := domain.TenantStatus(strings.ToUpper(strings.TrimSpace(string(*input.Status))))
		if status != domain.TenantStatusActive && status != domain.TenantStatusInactive {
			return fmt.Errorf("%w: status must be ACTIVE or INACTIVE", ErrInvalidInput)
		}
		input.Status = &status
		hasChange = true
	}
	if input.DefaultCountry != nil {
		country := strings.ToUpper(strings.TrimSpace(*input.DefaultCountry))
		if !tenantCountryRe.MatchString(country) {
			return fmt.Errorf("%w: default_country must be an ISO 3166-1 alpha-2 code", ErrInvalidInput)
		}
		input.DefaultCountry = &country
		hasChange = true
	}
	if input.Metadata != nil {
		if len(*input.Metadata) == 0 {
			metadata := json.RawMessage(`{}`)
			input.Metadata = &metadata
		}
		if len(*input.Metadata) > 8192 {
			return fmt.Errorf("%w: metadata must be 8192 bytes or less", ErrInvalidInput)
		}
		var metadata map[string]any
		if err := json.Unmarshal(*input.Metadata, &metadata); err != nil {
			return fmt.Errorf("%w: metadata must be a JSON object", ErrInvalidInput)
		}
		if metadata == nil {
			return fmt.Errorf("%w: metadata must be a JSON object", ErrInvalidInput)
		}
		hasChange = true
	}
	if !hasChange {
		return fmt.Errorf("%w: at least one tenant field is required", ErrInvalidInput)
	}
	return nil
}

func validateTenantMemberInput(input *domain.TenantMemberInput) error {
	if input == nil {
		return fmt.Errorf("%w: member payload is required", ErrInvalidInput)
	}
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.Auth0Subject = strings.TrimSpace(input.Auth0Subject)
	input.Role = domain.TenantMemberRole(strings.ToUpper(strings.TrimSpace(string(input.Role))))
	input.Status = domain.TenantMemberStatus(strings.ToUpper(strings.TrimSpace(string(input.Status))))

	if input.TenantID == "" {
		return fmt.Errorf("%w: tenant id is required", ErrInvalidInput)
	}
	if input.Auth0Subject == "" {
		return fmt.Errorf("%w: auth0_subject is required", ErrInvalidInput)
	}
	if len(input.Auth0Subject) > 255 {
		return fmt.Errorf("%w: auth0_subject is too long", ErrInvalidInput)
	}
	if input.Role == "" {
		input.Role = domain.TenantMemberRoleAdmin
	}
	if input.Role != domain.TenantMemberRoleAdmin && input.Role != domain.TenantMemberRoleViewer {
		return fmt.Errorf("%w: role must be TENANT_ADMIN or TENANT_VIEWER", ErrInvalidInput)
	}
	if input.Status == "" {
		input.Status = domain.TenantMemberStatusActive
	}
	if input.Status != domain.TenantMemberStatusActive && input.Status != domain.TenantMemberStatusInactive {
		return fmt.Errorf("%w: status must be ACTIVE or INACTIVE", ErrInvalidInput)
	}
	if input.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*input.Email))
		if email == "" {
			input.Email = nil
		} else {
			if len(email) > 255 || !strings.Contains(email, "@") {
				return fmt.Errorf("%w: email is invalid", ErrInvalidInput)
			}
			input.Email = &email
		}
	}
	if input.CreatedBy != nil {
		createdBy := strings.TrimSpace(*input.CreatedBy)
		if createdBy == "" {
			input.CreatedBy = nil
		} else {
			input.CreatedBy = &createdBy
		}
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
