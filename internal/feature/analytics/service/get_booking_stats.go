package analytics_service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	booking_events "github.com/poponyas/TicketRush/internal/feature/booking/events"
)

func (s *AnalyticsService) GetBookingStats(ctx context.Context, input BookingStatsInput) (BookingStatsReport, error) {
	if input.EventID == uuid.Nil || input.OrganizerID == uuid.Nil || input.UserID == uuid.Nil {
		return BookingStatsReport{}, fmt.Errorf("%w: event, organizer and user IDs are required", coreerrors.ErrInvalidArgument)
	}
	from, to := input.From.UTC(), input.To.UTC()
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return BookingStatsReport{}, fmt.Errorf("%w: from must be before to", coreerrors.ErrInvalidArgument)
	}
	// ClickHouse DateTime64(6) supports years 1900 through 2299. Leave room
	// at the upper end for rounding a sub-microsecond bound up in the repository.
	if from.Year() < 1900 || to.Add(time.Microsecond-time.Nanosecond).Year() >= 2300 {
		return BookingStatsReport{}, fmt.Errorf("%w: period is outside the supported timestamp range", coreerrors.ErrInvalidArgument)
	}

	// A role in the HTTP context is insufficient: verify current membership
	// and ownership of this specific event using the existing PostgreSQL query.
	if _, err := s.eventsRepository.GetEvent(ctx, input.EventID, input.OrganizerID, input.UserID); err != nil {
		return BookingStatsReport{}, fmt.Errorf("check event access for booking stats: %w", err)
	}
	rows, err := s.analyticsRepository.GetBookingStats(ctx, input.EventID, from, to)
	if err != nil {
		return BookingStatsReport{}, fmt.Errorf("get booking stats: %w", err)
	}

	report := BookingStatsReport{EventID: input.EventID, From: from, To: to}
	for _, row := range rows {
		stats := BookingStats{Bookings: row.Bookings, Tickets: row.Tickets}
		switch row.EventType {
		case booking_events.BookingConfirmedType:
			report.Confirmed = stats
		case booking_events.BookingCreatedType:
			report.Created = stats
		case booking_events.BookingCancelledType:
			report.Cancelled = stats
		case booking_events.BookingExpiredType:
			report.Expired = stats
		default:
			return BookingStatsReport{}, fmt.Errorf("unsupported booking stats event type %q", row.EventType)
		}
	}
	return report, nil
}
