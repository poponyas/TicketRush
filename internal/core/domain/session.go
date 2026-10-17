package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

type SessionStatus string

const (
	SessionDraft     SessionStatus = "draft"
	SessionScheduled SessionStatus = "scheduled"
	SessionCancelled SessionStatus = "cancelled"
	SessionCompleted SessionStatus = "completed"
)

// All dates describe instants; HTTP timestamps must include a UTC offset.
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

func NewSession(
	eventID uuid.UUID,
	venueID uuid.UUID,
	startsAt time.Time,
	endsAt *time.Time,
	salesStartsAt time.Time,
	salesEndAt time.Time,
) Session {
	id := uuid.New()
	return Session{
		ID:           id,
		EventID:      eventID,
		VenueID:      venueID,
		StartsAt:     startsAt,
		EndsAt:       endsAt,
		SalesStartAt: salesStartsAt,
		SalesEndAt:   salesEndAt,
		Status:       SessionDraft,
	}
}

func (s Session) Validate() error {
	if s.ID == uuid.Nil {
		return fmt.Errorf("session id is required: %w", core_errors.ErrInvalidArgument)
	}
	if s.EventID == uuid.Nil {
		return fmt.Errorf("event_id must be not nil: %w", core_errors.ErrInvalidArgument)
	}

	if s.VenueID == uuid.Nil {
		return fmt.Errorf("venue_id must be not nil: %w", core_errors.ErrInvalidArgument)
	}
	if s.StartsAt.IsZero() || s.SalesStartAt.IsZero() || s.SalesEndAt.IsZero() || s.CreatedAt.IsZero() {
		return fmt.Errorf("start, sales dates and creation time are required: %w", core_errors.ErrInvalidArgument)
	}
	if s.Status != SessionDraft && s.Status != SessionScheduled && s.Status != SessionCancelled && s.Status != SessionCompleted {
		return fmt.Errorf("invalid session status: %w", core_errors.ErrInvalidArgument)
	}

	if s.EndsAt != nil && !s.StartsAt.Before(*s.EndsAt) {
		return fmt.Errorf("end must be after start event session: %w", core_errors.ErrInvalidArgument)
	}

	if !s.SalesStartAt.Before(s.SalesEndAt) || s.SalesEndAt.After(s.StartsAt) {
		return fmt.Errorf("end must be after start sales: %w", core_errors.ErrInvalidArgument)
	}

	return nil
}
