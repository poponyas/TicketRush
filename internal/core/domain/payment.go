package domain

import (
	"github.com/google/uuid"
	"time"
)

type PaymentStatus string

const (
	PaymentPending   PaymentStatus = "pending"
	PaymentSucceeded PaymentStatus = "succeeded"
	PaymentFailed    PaymentStatus = "failed"
	PaymentRefunded  PaymentStatus = "refunded"
)

type Payment struct {
	ID                uuid.UUID
	OrderID           uuid.UUID
	Provider          string
	ProviderPaymentID string
	IdempotencyKey    string
	Amount            Money
	Status            PaymentStatus
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
