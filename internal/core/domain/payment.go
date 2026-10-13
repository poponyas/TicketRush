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
	ID                uuid.UUID `db:"id"`
	OrderID           uuid.UUID `db:"order_id"`
	Provider          string    `db:"provider"`
	ProviderPaymentID string    `db:"provider_payment_id"`
	IdempotencyKey    string    `db:"idempotency_key"`
	// Map the amount and currency explicitly when the payment schema is added.
	Amount    Money         `db:"-"`
	Status    PaymentStatus `db:"status"`
	CreatedAt time.Time     `db:"created_at"`
	UpdatedAt time.Time     `db:"updated_at"`
}
