package domain

import (
	"github.com/google/uuid"
	"time"
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
