package domain

import (
	"github.com/google/uuid"
	"time"
)

type SessionStatus string

const (
	SessionDraft     SessionStatus = "draft"
	SessionScheduled SessionStatus = "scheduled"
	SessionCancelled SessionStatus = "cancelled"
	SessionCompleted SessionStatus = "completed"
)

// All dates describe instants; the venue timezone controls display.
type Session struct {
	ID           uuid.UUID     `db:"id"`
	EventID      uuid.UUID     `db:"event_id"`
	VenueID      uuid.UUID     `db:"venue_id"`
	StartsAt     time.Time     `db:"starts_at"`
	EndsAt       *time.Time    `db:"ends_at"`
	SalesStartAt time.Time     `db:"sales_start_at"`
	SalesEndAt   time.Time     `db:"sales_end_at"`
	Status       SessionStatus `db:"status"`
	CreatedAt    time.Time     `db:"created_at"`
}
