package domain

import (
	"time"

	"github.com/google/uuid"
)

type BookingStatus string

const (
	BookingHeld      BookingStatus = "held"
	BookingConfirmed BookingStatus = "confirmed"
	BookingCancelled BookingStatus = "cancelled"
	BookingExpired   BookingStatus = "expired"
)

type Booking struct {
	ID        uuid.UUID     `db:"id"`
	UserID    uuid.UUID     `db:"user_id"`
	SessionID uuid.UUID     `db:"session_id"`
	Status    BookingStatus `db:"status"`
	ExpiresAt time.Time     `db:"expires_at"`
	CreatedAt time.Time     `db:"created_at"`
}

func NewBooking(
	userID uuid.UUID,
	sessionID uuid.UUID,
) Booking {
	id := uuid.New()
	return Booking{
		ID:        id,
		UserID:    userID,
		SessionID: sessionID,
		Status:    BookingHeld,
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
		CreatedAt: time.Now().UTC(),
	}
}
