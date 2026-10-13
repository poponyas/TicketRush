package domain

import "github.com/google/uuid"

// UnitPrice is captured at booking time and must not follow later tariff changes.
type BookingItem struct {
	ID           uuid.UUID `db:"id"`
	BookingID    uuid.UUID `db:"booking_id"`
	TicketTypeID uuid.UUID `db:"ticket_type_id"`
	Quantity     int       `db:"quantity"`
	// Map the amount and currency explicitly when the booking schema is added.
	UnitPrice Money `db:"-"`
}
