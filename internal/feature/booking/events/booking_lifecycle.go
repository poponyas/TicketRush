package booking_events

import "github.com/google/uuid"

const (
	BookingConfirmedType          = "booking.confirmed"
	BookingConfirmedSchemaVersion = 1
	BookingCancelledType          = "booking.cancelled"
	BookingCancelledSchemaVersion = 1
	BookingExpiredType            = "booking.expired"
	BookingExpiredSchemaVersion   = 1
)

// BookingLifecyclePayload is the shared v1 payload for booking lifecycle events.
type BookingLifecyclePayload struct {
	BookingID    uuid.UUID `json:"booking_id"`
	EventID      uuid.UUID `json:"event_id"`
	SessionID    uuid.UUID `json:"session_id"`
	TicketTypeID uuid.UUID `json:"ticket_type_id"`
	Quantity     int       `json:"quantity"`
}
