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
	ID           uuid.UUID
	EventID      uuid.UUID
	VenueID      uuid.UUID
	StartsAt     time.Time
	EndsAt       *time.Time
	SalesStartAt time.Time
	SalesEndAt   time.Time
	Status       SessionStatus
	CreatedAt    time.Time
}
