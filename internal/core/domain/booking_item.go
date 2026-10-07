package domain

import "github.com/google/uuid"

// UnitPrice is captured at booking time and must not follow later tariff changes.
type BookingItem struct {
	ID           uuid.UUID
	BookingID    uuid.UUID
	TicketTypeID uuid.UUID
	Quantity     int
	UnitPrice    Money
}
