package domain

import "time"

type BillingTransaction struct {
	ID        int
	MSISDN    string
	ProductID int
	Amount    float64
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}
