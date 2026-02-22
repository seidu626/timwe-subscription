package service

import (
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
	FetchNotifications(startDate, endDate time.Time, partnerRole, msisdn, entryChannel, notificationType string, page, pageSize int) (*domain.ListResponse, error)
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
	partnerRole := filters["partnerRole"]
	msisdn := filters["msisdn"]
	entryChannel := filters["entry_channel"]
	if entryChannel == "" {
		entryChannel = filters["entryChannel"]
	}
	notificationType := filters["type"]
	page, _ := strconv.Atoi(filters["page"])
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(filters["pageSize"])
	if pageSize < 1 {
		pageSize = 10
	}

	// Pass filters to the repository layer
	listResponse, err := s.repo.FetchNotifications(startDate, endDate, partnerRole, msisdn, entryChannel, notificationType, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("get notifications failed (page=%d pageSize=%d): %w", page, pageSize, err)
	}

	return listResponse, nil
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
