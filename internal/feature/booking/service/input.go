package booking_service

import "github.com/google/uuid"

type CreateBookingInput struct {
	EventID      uuid.UUID
	SessionID    uuid.UUID
	TicketTypeID uuid.UUID
	UserID       uuid.UUID
	Quantity     int
}
