package analytics_service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	"github.com/poponyas/TicketRush/internal/feature/analytics"
)

type AnalyticsService struct {
	analyticsRepository BookingStatsRepository
	eventsRepository    EventsRepository
}

type BookingStatsRepository interface {
	GetBookingStats(ctx context.Context, eventID uuid.UUID, from, to time.Time) ([]analytics.BookingStatsRow, error)
}

type EventsRepository interface {
	GetEvent(ctx context.Context, eventID, organizerID, userID uuid.UUID) (domain.Event, error)
}

func NewAnalyticsService(stats BookingStatsRepository, events EventsRepository) *AnalyticsService {
	return &AnalyticsService{analyticsRepository: stats, eventsRepository: events}
}

type BookingStatsInput struct {
	EventID     uuid.UUID
	OrganizerID uuid.UUID
	UserID      uuid.UUID
	From        time.Time
	To          time.Time
}

type BookingStats struct {
	Bookings uint64
	Tickets  uint64
}

type BookingStatsReport struct {
	EventID   uuid.UUID
	From      time.Time
	To        time.Time
	Created   BookingStats
	Confirmed BookingStats
	Cancelled BookingStats
	Expired   BookingStats
}
