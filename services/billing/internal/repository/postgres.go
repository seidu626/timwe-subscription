package repository

import (
	"database/sql"
	"fmt"
	"github.com/seidu626/subscription-manager/billing/internal/domain"
	"log"

	_ "github.com/lib/pq"
)

// BillingRepositoryInterface defines the contract for billing repository operations
type BillingRepositoryInterface interface {
	Save(tx *domain.BillingTransaction) error
	FindByMSISDN(msisdn string) ([]domain.BillingTransaction, error)
}

type BillingRepository struct {
	db *sql.DB
}

func NewBillingRepository(db *sql.DB) *BillingRepository {
	return &BillingRepository{db: db}
}

func (r *BillingRepository) Save(tx *domain.BillingTransaction) error {
	query := "INSERT INTO billing_transactions (msisdn, product_id, amount, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)"
	_, err := r.db.Exec(query, tx.MSISDN, tx.ProductID, tx.Amount, tx.Status, tx.CreatedAt, tx.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to save billing transaction: %w", err)
	}
	return nil
}

func (r *BillingRepository) FindByMSISDN(msisdn string) ([]domain.BillingTransaction, error) {
	query := "SELECT id, msisdn, product_id, amount, status, created_at, updated_at FROM billing_transactions WHERE msisdn = $1"
	rows, err := r.db.Query(query, msisdn)
	if err != nil {
		return nil, fmt.Errorf("failed to find transactions: %w", err)
	}
	defer rows.Close()

	var transactions []domain.BillingTransaction
	for rows.Next() {
		var tx domain.BillingTransaction
		if err := rows.Scan(&tx.ID, &tx.MSISDN, &tx.ProductID, &tx.Amount, &tx.Status, &tx.CreatedAt, &tx.UpdatedAt); err != nil {
			log.Println(err)
			continue
		}
		transactions = append(transactions, tx)
	}
	return transactions, nil
}
