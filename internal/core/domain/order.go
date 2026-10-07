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
	ID        uuid.UUID
	BookingID uuid.UUID
	UserID    uuid.UUID
	Total     Money
	Status    OrderStatus
	CreatedAt time.Time
	PaidAt    *time.Time
}
