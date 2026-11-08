package analytics_repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/feature/analytics"
)

type BookingEvent struct {
	MessageID     uuid.UUID
	BookingID     uuid.UUID
	EventID       uuid.UUID
	SessionID     uuid.UUID
	TicketTypeID  uuid.UUID
	EventType     string
	SchemaVersion uint32
	Quantity      uint32
	OccurredAt    time.Time
}

type BookingStatsRow = analytics.BookingStatsRow
