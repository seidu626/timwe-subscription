package service

import (
	"context"
	"fmt"
	"github.com/seidu626/subscription-manager/notification/internal/domain"
	"strconv"
	"strings"
	"time"
)

type NotificationService struct {
	repo notificationRepository
}

type notificationRepository interface {
	FetchNotifications(startDate, endDate time.Time, tenantID, channelID, partnerRole, msisdn, entryChannel, notificationType, sortBy, sortDir string, page, pageSize int) (*domain.ListResponse, error)
	TenantIDByKey(ctx context.Context, tenantKey string) (string, error)
	ChannelIDByKeys(ctx context.Context, tenantID, channelKey string) (string, error)
	Save(notification *domain.NotificationRequest) error
}

func NewNotificationService(repo notificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

// GetNotifications fetches notifications based on filters
func (s *NotificationService) GetNotifications(filters map[string]string) (*domain.ListResponse, error) {
	// Parse filter values
	startDate := parseFilterDate(filters["startDate"], false)
	endDate := parseFilterDate(filters["endDate"], true)
	tenantID := filters["tenantId"]
	channelID := filters["channelId"]
	partnerRole := filters["partnerRole"]
	msisdn := filters["msisdn"]
	entryChannel := filters["entry_channel"]
	if entryChannel == "" {
		entryChannel = filters["entryChannel"]
	}
	notificationType := filters["type"]
	sortBy := filters["sort_by"]
	if sortBy == "" {
		sortBy = filters["sortBy"]
	}
	sortDir := filters["sort_dir"]
	if sortDir == "" {
		sortDir = filters["sortDir"]
	}
	page, _ := strconv.Atoi(filters["page"])
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(filters["pageSize"])
	if pageSize < 1 {
		pageSize = 10
	}

	// Pass filters to the repository layer
	listResponse, err := s.repo.FetchNotifications(startDate, endDate, tenantID, channelID, partnerRole, msisdn, entryChannel, notificationType, sortBy, sortDir, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("get notifications failed (page=%d pageSize=%d): %w", page, pageSize, err)
	}

	return listResponse, nil
}

func (s *NotificationService) TenantIDByKey(ctx context.Context, tenantKey string) (string, error) {
	return s.repo.TenantIDByKey(ctx, tenantKey)
}

func (s *NotificationService) ChannelIDByKeys(ctx context.Context, tenantID, channelKey string) (string, error) {
	return s.repo.ChannelIDByKeys(ctx, tenantID, channelKey)
}

func (s *NotificationService) ProcessNotification(notification *domain.NotificationRequest) error {
	return s.repo.Save(notification)
}

func parseFilterDate(raw string, endOfDay bool) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}

	if len(raw) == len("2006-01-02") {
		if t, err := time.Parse("2006-01-02", raw); err == nil {
			if endOfDay {
				return t.Add(24*time.Hour - time.Nanosecond)
			}
			return t
		}
	}

	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}

	return time.Time{}
}
