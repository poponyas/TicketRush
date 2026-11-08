package booking_repository_test

import (
	"context"
	"encoding/json/v2"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	booking_events "github.com/poponyas/TicketRush/internal/feature/booking/events"
	"github.com/stretchr/testify/require"
)

func readBookingOutbox(t *testing.T, f *bookingFixture, bookingID uuid.UUID, eventType string) []domain.OutboxEvent {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := f.Pool.Query(ctx, `SELECT id,aggregate_id,type,schema_version,payload,created_at,published_at,attempts
  FROM outbox_events WHERE aggregate_id=$1 AND type=$2 ORDER BY created_at,id`, bookingID, eventType)
	require.NoError(t, err)
	events, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.OutboxEvent])
	require.NoError(t, err)
	return events
}

func assertBookingOutbox(t *testing.T, f *bookingFixture, bookingID uuid.UUID, eventType string, quantities map[uuid.UUID]int) []domain.OutboxEvent {
	t.Helper()
	events := readBookingOutbox(t, f, bookingID, eventType)
	require.Len(t, events, len(quantities))
	actual := make(map[uuid.UUID]int, len(events))
	for _, event := range events {
		require.NotEqual(t, uuid.Nil, event.ID)
		require.Equal(t, bookingID, event.AggregateID)
		require.Equal(t, eventType, event.Type)
		require.Equal(t, 1, event.SchemaVersion)
		require.Nil(t, event.PublishedAt)
		require.Zero(t, event.Attempts)
		var payload booking_events.BookingLifecyclePayload
		require.NoError(t, json.Unmarshal(event.Payload, &payload))
		require.Equal(t, bookingID, payload.BookingID)
		require.Equal(t, f.eventID, payload.EventID)
		require.Equal(t, f.sessionID, payload.SessionID)
		actual[payload.TicketTypeID] = payload.Quantity
	}
	require.Equal(t, quantities, actual)
	return events
}

func TestBookingLifecycleOutboxPostgres(t *testing.T) {
	for _, eventType := range []string{booking_events.BookingCancelledType, booking_events.BookingExpiredType} {
		t.Run(eventType, func(t *testing.T) {
			f := newExpirationFixture(t, 5)
			draft := domain.NewBooking(f.UserID, f.sessionID)
			draft.CreatedAt = f.now.Add(-2 * time.Hour)
			b, err := f.repo.CreateBooking(context.Background(), draft, f.eventID, f.ticketTypeID, 2)
			require.NoError(t, err)
			quantities := map[uuid.UUID]int{f.ticketTypeID: 2}
			created := assertBookingOutbox(t, f, b.ID, booking_events.BookingCreatedType, quantities)
			require.WithinDuration(t, b.CreatedAt, created[0].CreatedAt, time.Microsecond)

			before := time.Now().UTC().Add(-time.Millisecond)
			if eventType == booking_events.BookingCancelledType {
				_, err = f.repo.CancelBooking(context.Background(), b.ID, f.UserID)
				require.NoError(t, err)
				assertBookingStatus(t, f, b.ID, domain.BookingCancelled)
			} else {
				markOverdue(t, f, b)
				count, err := f.repo.ExpireBookings(context.Background(), 100)
				require.NoError(t, err)
				require.Equal(t, 1, count)
				assertBookingStatus(t, f, b.ID, domain.BookingExpired)
			}
			events := assertBookingOutbox(t, f, b.ID, eventType, quantities)
			require.False(t, events[0].CreatedAt.Before(before))
			require.False(t, events[0].CreatedAt.After(time.Now().UTC().Add(time.Millisecond)))
			require.True(t, events[0].CreatedAt.After(b.CreatedAt), "transition time must not reuse booking creation time")
			f.assertInventory(t, 5, 0)

			if eventType == booking_events.BookingCancelledType {
				_, err = f.repo.CancelBooking(context.Background(), b.ID, f.UserID)
				require.NoError(t, err)
			} else {
				count, err := f.repo.ExpireBookings(context.Background(), 100)
				require.NoError(t, err)
				require.Zero(t, count)
			}
			repeated := assertBookingOutbox(t, f, b.ID, eventType, quantities)
			require.Equal(t, events[0].ID, repeated[0].ID)
			require.Equal(t, created[0].ID, assertBookingOutbox(t, f, b.ID, booking_events.BookingCreatedType, quantities)[0].ID)
		})
	}
}

func TestBookingOutboxFailureRollsBackPostgres(t *testing.T) {
	t.Run("creation", func(t *testing.T) {
		f := newExpirationFixture(t, 5)
		draft := domain.NewBooking(f.UserID, f.sessionID)
		unblock := rejectBookingOutbox(t, f, draft.ID, booking_events.BookingCreatedType)
		_, err := f.repo.CreateBooking(context.Background(), draft, f.eventID, f.ticketTypeID, 2)
		require.ErrorContains(t, err, "outbox event")
		f.assertNoBooking(t, draft.ID)
		f.assertInventory(t, 5, 0)
		require.Empty(t, readBookingOutbox(t, f, draft.ID, booking_events.BookingCreatedType))
		unblock()
		_, err = f.repo.CreateBooking(context.Background(), draft, f.eventID, f.ticketTypeID, 2)
		require.NoError(t, err)
		assertBookingOutbox(t, f, draft.ID, booking_events.BookingCreatedType, map[uuid.UUID]int{f.ticketTypeID: 2})
	})
	t.Run("cancellation", func(t *testing.T) {
		f := newExpirationFixture(t, 5)
		b := heldBooking(t, f, 2)
		unblock := rejectBookingOutbox(t, f, b.ID, booking_events.BookingCancelledType)
		_, err := f.repo.CancelBooking(context.Background(), b.ID, f.UserID)
		require.ErrorContains(t, err, "outbox event")
		assertBookingStatus(t, f, b.ID, domain.BookingHeld)
		f.assertInventory(t, 3, 2)
		require.Empty(t, readBookingOutbox(t, f, b.ID, booking_events.BookingCancelledType))
		assertBookingOutbox(t, f, b.ID, booking_events.BookingCreatedType, map[uuid.UUID]int{f.ticketTypeID: 2})
		unblock()
		_, err = f.repo.CancelBooking(context.Background(), b.ID, f.UserID)
		require.NoError(t, err)
		assertBookingOutbox(t, f, b.ID, booking_events.BookingCancelledType, map[uuid.UUID]int{f.ticketTypeID: 2})
	})
	t.Run("expiration batch", func(t *testing.T) {
		f := newExpirationFixture(t, 5)
		a, b := heldBooking(t, f, 1), heldBooking(t, f, 1)
		markOverdue(t, f, a)
		markOverdue(t, f, b)
		// The helper orders by booking ID: fail the second insert to prove that
		// the first outbox insert rolls back along with every status and counter.
		failID := a.ID
		if b.ID.String() > a.ID.String() {
			failID = b.ID
		}
		unblock := rejectBookingOutbox(t, f, failID, booking_events.BookingExpiredType)
		count, err := f.repo.ExpireBookings(context.Background(), 100)
		require.ErrorContains(t, err, "outbox event")
		require.Zero(t, count)
		f.assertInventory(t, 3, 2)
		for _, id := range []uuid.UUID{a.ID, b.ID} {
			assertBookingStatus(t, f, id, domain.BookingHeld)
			require.Empty(t, readBookingOutbox(t, f, id, booking_events.BookingExpiredType))
		}
		unblock()
		count, err = f.repo.ExpireBookings(context.Background(), 100)
		require.NoError(t, err)
		require.Equal(t, 2, count)
		f.assertInventory(t, 5, 0)
		for _, id := range []uuid.UUID{a.ID, b.ID} {
			assertBookingOutbox(t, f, id, booking_events.BookingExpiredType, map[uuid.UUID]int{f.ticketTypeID: 1})
		}
	})
}

func TestExpireBookingMultipleTariffsOutboxPostgres(t *testing.T) {
	f := newExpirationFixture(t, 5)
	b := heldBooking(t, f, 2)
	second := uuid.New()
	f.Exec(t, `INSERT INTO booking_item(id,booking_id,ticket_type_id,quantity,unit_price,currency)
  VALUES($1,$2,$3,1,1500.25,'RUB')`, uuid.New(), b.ID, f.ticketTypeID)
	f.Exec(t, `UPDATE ticket_types SET available=2,held=3 WHERE id=$1`, f.ticketTypeID)
	f.Exec(t, `INSERT INTO ticket_types(id,session_id,name,price,currency,capacity,available,held,sold)
  VALUES($1,$2,'Second',100,'RUB',1,0,1,0)`, second, f.sessionID)
	f.Exec(t, `INSERT INTO booking_item(id,booking_id,ticket_type_id,quantity,unit_price,currency)
  VALUES($1,$2,$3,1,100,'RUB')`, uuid.New(), b.ID, second)
	markOverdue(t, f, b)
	count, err := f.repo.ExpireBookings(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 1, count, "one booking expired even though two tariff events are emitted")
	f.assertInventory(t, 5, 0)
	assertBookingOutbox(t, f, b.ID, booking_events.BookingExpiredType, map[uuid.UUID]int{f.ticketTypeID: 3, second: 1})
}

func rejectBookingOutbox(t *testing.T, f *bookingFixture, bookingID uuid.UUID, eventType string) func() {
	t.Helper()
	constraint := pgx.Identifier{"test_outbox_failure_" + strings.ReplaceAll(bookingID.String(), "-", "")}.Sanitize()
	f.Exec(t, `ALTER TABLE outbox_events ADD CONSTRAINT `+constraint+
		` CHECK(aggregate_id <> '`+bookingID.String()+`'::uuid OR type <> '`+eventType+`')`)
	unblock := func() { f.Exec(t, `ALTER TABLE outbox_events DROP CONSTRAINT IF EXISTS `+constraint) }
	t.Cleanup(unblock)
	return unblock
}
