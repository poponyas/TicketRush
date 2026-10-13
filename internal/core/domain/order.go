package domain

import (
	"github.com/google/uuid"
	"time"
)

type OrderStatus string

const (
	OrderPending        OrderStatus = "pending"
	OrderPaid           OrderStatus = "paid"
	OrderCancelled      OrderStatus = "cancelled"
	OrderRefundRequired OrderStatus = "refund_required"
	OrderRefunded       OrderStatus = "refunded"
)

type Order struct {
	ID        uuid.UUID `db:"id"`
	BookingID uuid.UUID `db:"booking_id"`
	UserID    uuid.UUID `db:"user_id"`
	// Map the amount and currency explicitly when the order schema is added.
	Total     Money       `db:"-"`
	Status    OrderStatus `db:"status"`
	CreatedAt time.Time   `db:"created_at"`
	PaidAt    *time.Time  `db:"paid_at"`
}
