package service

import (
	"github.com/seidu626/subscription-manager/billing/internal/domain"
	"github.com/seidu626/subscription-manager/billing/internal/repository"
	"time"
)

type BillingService struct {
	repo repository.BillingRepositoryInterface
}

func NewBillingService(repo repository.BillingRepositoryInterface) *BillingService {
	return &BillingService{repo: repo}
}

func (s *BillingService) ProcessPayment(msisdn string, productID int, amount float64) (*domain.BillingTransaction, error) {
	tx := &domain.BillingTransaction{
		MSISDN:    msisdn,
		ProductID: productID,
		Amount:    amount,
		Status:    "completed",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := s.repo.Save(tx)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func (s *BillingService) FindByMSISDN(msisdn string) ([]domain.BillingTransaction, error) {
	return s.repo.FindByMSISDN(msisdn)
}
