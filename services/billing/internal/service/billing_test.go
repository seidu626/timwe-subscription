package service

import (
	"github.com/seidu626/subscription-manager/billing/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

// BillingRepositoryMock implements repository.BillingRepositoryInterface
type BillingRepositoryMock struct {
	mock.Mock
}

func (m *BillingRepositoryMock) Save(tx *domain.BillingTransaction) error {
	args := m.Called(tx)
	return args.Error(0)
}

func (m *BillingRepositoryMock) FindByMSISDN(msisdn string) ([]domain.BillingTransaction, error) {
	args := m.Called(msisdn)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.BillingTransaction), args.Error(1)
}

func TestProcessPayment(t *testing.T) {
	repoMock := new(BillingRepositoryMock)
	repoMock.On("Save", mock.Anything).Return(nil)

	svc := NewBillingService(repoMock)
	tx, err := svc.ProcessPayment("233577250333", 101, 100.0)

	assert.NoError(t, err)
	assert.NotNil(t, tx)
	assert.Equal(t, "233577250333", tx.MSISDN)
}
