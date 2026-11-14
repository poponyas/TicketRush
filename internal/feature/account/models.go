// Package account contains read models for the signed-in user's cabinet.
package account

import (
	"github.com/google/uuid"
	"time"
)

type Organization struct {
	ID   uuid.UUID `json:"id" db:"id"`
	Name string    `json:"name" db:"name"`
	Role string    `json:"role" db:"role"`
}
type Profile struct {
	ID            uuid.UUID      `json:"id"`
	Email         string         `json:"email"`
	DisplayName   string         `json:"display_name"`
	VenueOwner    bool           `json:"venue_owner"`
	Organizations []Organization `json:"organizations"`
}
type Booking struct {
	ID        uuid.UUID `json:"id" db:"id"`
	EventID   uuid.UUID `json:"event_id" db:"event_id"`
	SessionID uuid.UUID `json:"session_id" db:"session_id"`
	EventName string    `json:"event_name" db:"event_name"`
	VenueName string    `json:"venue_name" db:"venue_name"`
	City      string    `json:"city" db:"city"`
	Status    string    `json:"status" db:"status"`
	StartsAt  time.Time `json:"starts_at" db:"starts_at"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	Quantity  int64     `json:"quantity" db:"quantity"`
	Total     string    `json:"total" db:"total"`
	Currency  string    `json:"currency" db:"currency"`
}
type BookingPage struct {
	Items  []Booking `json:"items"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
}
