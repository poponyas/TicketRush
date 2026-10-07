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
	ID        uuid.UUID
	UserID    uuid.UUID
	SessionID uuid.UUID
	Status    BookingStatus
	ExpiresAt time.Time
	CreatedAt time.Time
}
