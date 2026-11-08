package analytics_repository_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	repository "github.com/poponyas/TicketRush/internal/feature/analytics/repository"
	booking_events "github.com/poponyas/TicketRush/internal/feature/booking/events"
	"github.com/stretchr/testify/require"
)

// This test creates and removes its own database; it never uses application data.
func newClickHouseAnalytics(t *testing.T) *repository.Analytics {
	t.Helper()
	addr := os.Getenv("TICKETRUSH_TEST_CLICKHOUSE_ADDR")
	if addr == "" {
		t.Skip("set TICKETRUSH_TEST_CLICKHOUSE_ADDR (and optionally _USER, _PASSWORD) for ClickHouse integration tests")
	}
	user := os.Getenv("TICKETRUSH_TEST_CLICKHOUSE_USER")
	if user == "" {
		user = "default"
	}
	auth := clickhouse.Auth{Database: "default", Username: user, Password: os.Getenv("TICKETRUSH_TEST_CLICKHOUSE_PASSWORD")}
	admin, err := clickhouse.Open(&clickhouse.Options{Addr: []string{addr}, Auth: auth, DialTimeout: 5 * time.Second})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, admin.Close()) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	require.NoError(t, admin.Ping(ctx))
	database := "analytics_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec(ctx, "CREATE DATABASE "+database))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		require.NoError(t, admin.Exec(ctx, "DROP DATABASE "+database))
	})
	migration, err := os.ReadFile("../../../../clickhouse/migrations/001_booking_events.up.sql")
	require.NoError(t, err)
	require.NoError(t, admin.Exec(ctx, strings.ReplaceAll(string(migration), "ticketrush.", database+".")))
	auth.Database = database
	conn, err := clickhouse.Open(&clickhouse.Options{Addr: []string{addr}, Auth: auth, DialTimeout: 5 * time.Second})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	return repository.NewAnalytics(conn, 5*time.Second)
}

func TestGetBookingStatsClickHouse(t *testing.T) {
	repo := newClickHouseAnalytics(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	eventID, a, b, old, tariffA, tariffB := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	sessionID := uuid.New()
	from := time.Date(2026, 11, 8, 12, 0, 0, 123456000, time.UTC)
	to := from.Add(time.Hour)
	makeEvent := func(booking, tariff uuid.UUID, kind string, qty uint32, at time.Time) repository.BookingEvent {
		return repository.BookingEvent{
			MessageID: uuid.New(), BookingID: booking, EventID: eventID, SessionID: sessionID,
			TicketTypeID: tariff, EventType: kind, SchemaVersion: 1, Quantity: qty, OccurredAt: at,
		}
	}
	events := []repository.BookingEvent{
		makeEvent(a, tariffA, booking_events.BookingCreatedType, 2, from),
		makeEvent(a, tariffB, booking_events.BookingCreatedType, 1, from),
		// A separate booking with the same quantity must still contribute its tickets.
		makeEvent(b, tariffA, booking_events.BookingCreatedType, 2, from.Add(time.Microsecond)),
		makeEvent(a, tariffA, booking_events.BookingCancelledType, 2, from.Add(time.Minute)),
		makeEvent(a, tariffB, booking_events.BookingCancelledType, 1, from.Add(time.Minute)),
		makeEvent(b, tariffA, booking_events.BookingExpiredType, 2, to.Add(-time.Microsecond)),
		// Creation is outside the period, but its cancellation belongs in this report.
		makeEvent(old, tariffA, booking_events.BookingCreatedType, 4, from.Add(-time.Microsecond)),
		makeEvent(old, tariffA, booking_events.BookingCancelledType, 4, from.Add(2*time.Minute)),
		// The upper bound is exclusive.
		makeEvent(uuid.New(), tariffA, booking_events.BookingExpiredType, 100, to),
	}
	foreign := makeEvent(uuid.New(), tariffA, booking_events.BookingCreatedType, 100, from)
	foreign.EventID = uuid.New()
	events = append(events, foreign)
	require.NoError(t, repo.InsertBatch(ctx, events))
	// Simulate Kafka redelivery after a successful insert but a failed offset commit.
	require.NoError(t, repo.InsertBatch(ctx, events))

	location := time.FixedZone("MSK", 3*60*60)
	stats, err := repo.GetBookingStats(ctx, eventID, from.In(location), to.In(location))
	require.NoError(t, err)
	require.Equal(t, []repository.BookingStatsRow{
		{EventType: booking_events.BookingCancelledType, Bookings: 2, Tickets: 7},
		{EventType: booking_events.BookingCreatedType, Bookings: 2, Tickets: 5},
		{EventType: booking_events.BookingExpiredType, Bookings: 1, Tickets: 2},
	}, stats)

	t.Run("sub-microsecond bounds", func(t *testing.T) {
		stats, err := repo.GetBookingStats(ctx, eventID, from.Add(time.Nanosecond), from.Add(time.Microsecond+time.Nanosecond))
		require.NoError(t, err)
		require.Equal(t, []repository.BookingStatsRow{{EventType: booking_events.BookingCreatedType, Bookings: 1, Tickets: 2}}, stats)
	})
	t.Run("empty report", func(t *testing.T) {
		stats, err := repo.GetBookingStats(ctx, uuid.New(), from, to)
		require.NoError(t, err)
		require.NotNil(t, stats)
		require.Empty(t, stats)
	})
}
