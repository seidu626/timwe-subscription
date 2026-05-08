package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

var allowedBackgroundImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

var ErrCampaignAssetStorageUnavailable = errors.New("campaign asset storage unavailable")

var sanitizeAssetSegmentRe = regexp.MustCompile(`[^a-z0-9\-]+`)

type CampaignAssetStorageConfig struct {
	Enabled            bool
	Endpoint           string
	Bucket             string
	Region             string
	AccessKeyID        string
	SecretAccessKey    string
	UseSSL             bool
	PublicBaseURL      string
	KeyPrefix          string
	MaxUploadSizeBytes int64
	PresignExpiry      time.Duration
}

type CampaignAssetUploadRequest struct {
	TenantNamespace string
	CampaignSlug    string
	FileName        string
	ContentType     string
	SizeBytes       int64
}

type CampaignAssetUploadResponse struct {
	UploadURL           string   `json:"upload_url"`
	AssetURL            string   `json:"asset_url"`
	ObjectKey           string   `json:"object_key"`
	ExpiresInSeconds    int64    `json:"expires_in_seconds"`
	MaxSizeBytes        int64    `json:"max_size_bytes"`
	AllowedContentTypes []string `json:"allowed_content_types"`
}

type CampaignAssetService struct {
	cfg    CampaignAssetStorageConfig
	client *minio.Client
	logger *zap.Logger
}

func NewCampaignAssetService(cfg CampaignAssetStorageConfig, logger *zap.Logger) (*CampaignAssetService, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, fmt.Errorf("campaign asset storage endpoint is required")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("campaign asset storage bucket is required")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return nil, fmt.Errorf("campaign asset storage credentials are required")
	}
	if cfg.MaxUploadSizeBytes <= 0 {
		cfg.MaxUploadSizeBytes = 2 * 1024 * 1024
	}
	if cfg.PresignExpiry <= 0 {
		cfg.PresignExpiry = 10 * time.Minute
	}

	endpoint := normalizeStorageEndpoint(cfg.Endpoint)
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init campaign asset storage client: %w", err)
	}

	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "campaign-backgrounds"
	}
	cfg.KeyPrefix = strings.Trim(cfg.KeyPrefix, "/")

	return &CampaignAssetService{
		cfg:    cfg,
		client: client,
		logger: logger,
	}, nil
}

func (s *CampaignAssetService) Enabled() bool {
	return s != nil && s.client != nil
}

func (s *CampaignAssetService) PresignBackgroundUpload(ctx context.Context, req CampaignAssetUploadRequest) (*CampaignAssetUploadResponse, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("campaign asset storage is not configured")
	}

	contentType := strings.ToLower(strings.TrimSpace(req.ContentType))
	ext, ok := allowedBackgroundImageTypes[contentType]
	if !ok {
		return nil, fmt.Errorf("content_type must be one of: %s", strings.Join(s.allowedContentTypes(), ", "))
	}

	if req.SizeBytes <= 0 {
		return nil, fmt.Errorf("size_bytes must be greater than 0")
	}
	if req.SizeBytes > s.cfg.MaxUploadSizeBytes {
		return nil, fmt.Errorf("size_bytes exceeds max of %d bytes", s.cfg.MaxUploadSizeBytes)
	}

	tenantSegment := sanitizeAssetSegment(req.TenantNamespace)
	if tenantSegment == "" {
		return nil, fmt.Errorf("tenant namespace is required")
	}
	slugSegment := sanitizeAssetSegment(req.CampaignSlug)
	if slugSegment == "" {
		slugSegment = "campaign"
	}

	fileName := strings.TrimSpace(req.FileName)
	if fileName != "" {
		if err := validateAssetFileName(fileName); err != nil {
			return nil, err
		}
		lower := strings.ToLower(fileName)
		if strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".jpg") {
			ext = ".jpg"
		}
		if strings.HasSuffix(lower, ".png") {
			ext = ".png"
		}
		if strings.HasSuffix(lower, ".webp") {
			ext = ".webp"
		}
	}

	objectKey := buildBackgroundObjectKey(s.cfg.KeyPrefix, tenantSegment, slugSegment, time.Now(), uuid.NewString(), ext)

	uploadURL, err := s.client.PresignedPutObject(ctx, s.cfg.Bucket, objectKey, s.cfg.PresignExpiry)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create presigned upload URL", ErrCampaignAssetStorageUnavailable)
	}

	assetURL := s.buildAssetURL(objectKey)
	resp := &CampaignAssetUploadResponse{
		UploadURL:           uploadURL.String(),
		AssetURL:            assetURL,
		ObjectKey:           objectKey,
		ExpiresInSeconds:    int64(s.cfg.PresignExpiry.Seconds()),
		MaxSizeBytes:        s.cfg.MaxUploadSizeBytes,
		AllowedContentTypes: s.allowedContentTypes(),
	}

	s.logger.Info("Generated campaign background upload URL",
		zap.String("campaign_slug", req.CampaignSlug),
		zap.String("object_key", objectKey),
		zap.String("content_type", contentType),
		zap.Int64("size_bytes", req.SizeBytes),
	)

	return resp, nil
}

func (s *CampaignAssetService) allowedContentTypes() []string {
	out := make([]string, 0, len(allowedBackgroundImageTypes))
	for t := range allowedBackgroundImageTypes {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

func (s *CampaignAssetService) buildAssetURL(objectKey string) string {
	if strings.TrimSpace(s.cfg.PublicBaseURL) != "" {
		base, err := url.Parse(strings.TrimSpace(s.cfg.PublicBaseURL))
		if err == nil {
			base.Path = path.Join(strings.TrimSuffix(base.Path, "/"), objectKey)
			return base.String()
		}
	}

	scheme := "http"
	if s.cfg.UseSSL {
		scheme = "https"
	}

	endpoint := normalizeStorageEndpoint(s.cfg.Endpoint)
	return fmt.Sprintf("%s://%s/%s/%s", scheme, endpoint, s.cfg.Bucket, objectKey)
}

func normalizeStorageEndpoint(endpoint string) string {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		u, err := url.Parse(trimmed)
		if err == nil && u.Host != "" {
			return u.Host
		}
	}
	return strings.Trim(trimmed, "/")
}

func sanitizeAssetSegment(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	normalized = sanitizeAssetSegmentRe.ReplaceAllString(normalized, "-")
	normalized = strings.Trim(normalized, "-")
	if len(normalized) > 80 {
		normalized = normalized[:80]
	}
	return normalized
}

func validateAssetFileName(fileName string) error {
	if strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") || strings.Contains(fileName, "..") {
		return fmt.Errorf("file_name must be a simple file name")
	}
	for _, r := range fileName {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("file_name contains control characters")
		}
	}
	return nil
}

func buildBackgroundObjectKey(prefix, tenantSegment, campaignSegment string, now time.Time, id string, ext string) string {
	return fmt.Sprintf("%s/tenants/%s/%s/%d-%s%s", strings.Trim(prefix, "/"), tenantSegment, campaignSegment, now.Unix(), id, ext)
}
