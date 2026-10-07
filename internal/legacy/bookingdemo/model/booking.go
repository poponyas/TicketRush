package domain

import "github.com/google/uuid"

type Booking struct {
	ID       uuid.UUID `json:"id"`
	EventID  uuid.UUID `json:"event_id"`
	UserID   string    `json:"user_id"`
	Quantity int       `json:"quantity"`
	Status   string    `json:"status"`
}

func NewBooking(eventID uuid.UUID, userID string, quantity int, status string) Booking {
	id := uuid.New()
	return Booking{
		ID:       id,
		EventID:  eventID,
		UserID:   userID,
		Quantity: quantity,
		Status:   status,
	}
}
