package analytics_service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	"github.com/poponyas/TicketRush/internal/feature/analytics"
	service "github.com/poponyas/TicketRush/internal/feature/analytics/service"
	booking_events "github.com/poponyas/TicketRush/internal/feature/booking/events"
	"github.com/stretchr/testify/require"
)

type eventsStub struct {
	get func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Event, error)
}

func (s eventsStub) GetEvent(ctx context.Context, event, org, user uuid.UUID) (domain.Event, error) {
	return s.get(ctx, event, org, user)
}

type statsStub struct {
	get func(context.Context, uuid.UUID, time.Time, time.Time) ([]analytics.BookingStatsRow, error)
}

func (s statsStub) GetBookingStats(ctx context.Context, event uuid.UUID, from, to time.Time) ([]analytics.BookingStatsRow, error) {
	return s.get(ctx, event, from, to)
}

func statsInput() service.BookingStatsInput {
	from := time.Date(2026, 11, 8, 12, 0, 0, 0, time.FixedZone("MSK", 3*60*60))
	return service.BookingStatsInput{
		EventID: uuid.New(), OrganizerID: uuid.New(), UserID: uuid.New(),
		From: from, To: from.Add(time.Hour),
	}
}

func TestGetBookingStatsChecksAccessBeforeAnalytics(t *testing.T) {
	input := statsInput()
	var calls []string
	events := eventsStub{get: func(ctx context.Context, event, org, user uuid.UUID) (domain.Event, error) {
		calls = append(calls, "access")
		require.Equal(t, input.EventID, event)
		require.Equal(t, input.OrganizerID, org)
		require.Equal(t, input.UserID, user)
		return domain.Event{ID: event, OrganizerID: org}, nil
	}}
	stats := statsStub{get: func(ctx context.Context, event uuid.UUID, from, to time.Time) ([]analytics.BookingStatsRow, error) {
		calls = append(calls, "analytics")
		require.Equal(t, input.EventID, event)
		require.Equal(t, input.From.UTC(), from)
		require.Equal(t, input.To.UTC(), to)
		return []analytics.BookingStatsRow{
			{EventType: booking_events.BookingCreatedType, Bookings: 4, Tickets: 9},
			{EventType: booking_events.BookingCancelledType, Bookings: 1, Tickets: 2},
		}, nil
	}}
	report, err := service.NewAnalyticsService(stats, events).GetBookingStats(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, []string{"access", "analytics"}, calls)
	require.Equal(t, input.EventID, report.EventID)
	require.Equal(t, input.From.UTC(), report.From)
	require.Equal(t, input.To.UTC(), report.To)
	require.Equal(t, service.BookingStats{Bookings: 4, Tickets: 9}, report.Created)
	require.Equal(t, service.BookingStats{Bookings: 1, Tickets: 2}, report.Cancelled)
	require.Equal(t, service.BookingStats{}, report.Expired)
}

func TestGetBookingStatsEmptyPeriod(t *testing.T) {
	input := statsInput()
	events := eventsStub{get: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Event, error) {
		return domain.Event{}, nil
	}}
	stats := statsStub{get: func(context.Context, uuid.UUID, time.Time, time.Time) ([]analytics.BookingStatsRow, error) {
		return nil, nil
	}}
	report, err := service.NewAnalyticsService(stats, events).GetBookingStats(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, service.BookingStats{}, report.Created)
	require.Equal(t, service.BookingStats{}, report.Cancelled)
	require.Equal(t, service.BookingStats{}, report.Expired)
}

func TestGetBookingStatsRejectsInvalidInputBeforeRepositories(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*service.BookingStatsInput)
	}{
		{"no event", func(i *service.BookingStatsInput) { i.EventID = uuid.Nil }},
		{"no organizer", func(i *service.BookingStatsInput) { i.OrganizerID = uuid.Nil }},
		{"no user", func(i *service.BookingStatsInput) { i.UserID = uuid.Nil }},
		{"no from", func(i *service.BookingStatsInput) { i.From = time.Time{} }},
		{"no to", func(i *service.BookingStatsInput) { i.To = time.Time{} }},
		{"equal", func(i *service.BookingStatsInput) { i.To = i.From }},
		{"reversed", func(i *service.BookingStatsInput) { i.To = i.From.Add(-time.Second) }},
		{"before supported range", func(i *service.BookingStatsInput) { i.From = time.Date(1800, 1, 1, 0, 0, 0, 0, time.UTC) }},
		{"after supported range", func(i *service.BookingStatsInput) { i.To = time.Date(2300, 1, 1, 0, 0, 0, 0, time.UTC) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := statsInput()
			tc.edit(&input)
			// Any repository call panics: validation must run first.
			_, err := service.NewAnalyticsService(statsStub{}, eventsStub{}).GetBookingStats(context.Background(), input)
			require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
		})
	}
}

func TestGetBookingStatsRepositoryErrors(t *testing.T) {
	for _, expected := range []error{coreerrors.ErrNotFound, coreerrors.ErrForbidden, errors.New("postgres unavailable")} {
		t.Run(expected.Error(), func(t *testing.T) {
			events := eventsStub{get: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Event, error) {
				return domain.Event{}, fmt.Errorf("get event: %w", expected)
			}}
			// Analytics must never be queried when access cannot be established.
			_, err := service.NewAnalyticsService(statsStub{}, events).GetBookingStats(context.Background(), statsInput())
			require.ErrorIs(t, err, expected)
		})
	}
	t.Run("clickhouse unavailable", func(t *testing.T) {
		expected := errors.New("clickhouse unavailable")
		events := eventsStub{get: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Event, error) {
			return domain.Event{}, nil
		}}
		stats := statsStub{get: func(context.Context, uuid.UUID, time.Time, time.Time) ([]analytics.BookingStatsRow, error) {
			return nil, expected
		}}
		_, err := service.NewAnalyticsService(stats, events).GetBookingStats(context.Background(), statsInput())
		require.ErrorIs(t, err, expected)
	})
}
