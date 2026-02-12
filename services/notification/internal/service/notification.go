package service

import (
	"github.com/seidu626/subscription-manager/notification/internal/domain"
	"github.com/seidu626/subscription-manager/notification/internal/repository"
	"strconv"
	"time"
)

type NotificationService struct {
	repo *repository.NotificationRepository
}

func NewNotificationService(repo *repository.NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

// GetNotifications fetches notifications based on filters
func (s *NotificationService) GetNotifications(filters map[string]string) (*domain.ListResponse, error) {
	// Parse filter values
	startDate, _ := time.Parse("2006-01-02", filters["startDate"])
	endDate, _ := time.Parse("2006-01-02", filters["endDate"])
	partnerRole := filters["partnerRole"]
	msisdn := filters["msisdn"]
	entryChannel := filters["entry_channel"]
	page, _ := strconv.Atoi(filters["page"])
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(filters["pageSize"])
	if pageSize < 1 {
		pageSize = 10
	}

	// Pass filters to the repository layer
	return s.repo.FetchNotifications(startDate, endDate, partnerRole, msisdn, entryChannel, page, pageSize)
}

func (s *NotificationService) ProcessNotification(notification *domain.NotificationRequest) error {
	return s.repo.Save(notification)
}
