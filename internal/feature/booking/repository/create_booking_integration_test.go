package booking_repository_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	repository "github.com/poponyas/TicketRush/internal/feature/booking/repository"
	"github.com/poponyas/TicketRush/internal/feature/events/testutil"
	"github.com/stretchr/testify/require"
)

type bookingFixture struct {
	*testutil.Fixture
	repo                             *repository.BookingRepository
	eventID, sessionID, ticketTypeID uuid.UUID
	now                              time.Time
}

func newBookingFixture(t *testing.T, capacity int) *bookingFixture {
	t.Helper()
	f := testutil.New(t)
	now := time.Now().UTC().Truncate(time.Second)
	event := f.Event(t, f.OrgID, "Booking test")
	session := f.Session(t, event.ID)
	f.Exec(t, `UPDATE events SET status='published' WHERE id=$1`, event.ID)
	f.Exec(t, `UPDATE sessions SET status='scheduled',starts_at=$1,sales_start_at=$2,sales_end_at=$3 WHERE id=$4`,
		now.Add(48*time.Hour), now.Add(-time.Hour), now.Add(24*time.Hour), session)
	ticketType := uuid.New()
	f.Exec(t, `INSERT INTO ticket_types(id,session_id,name,price,currency,capacity,available,held,sold)
  VALUES($1,$2,'Standard',1500.25,'RUB',$3,$3,0,0)`, ticketType, session, capacity)
	return &bookingFixture{Fixture: f, repo: repository.NewBookingRepository(&postgres.Pool{Pool: f.Pool, OpTimeout: 5 * time.Second}),
		eventID: event.ID, sessionID: session, ticketTypeID: ticketType, now: now}
}
func (f *bookingFixture) assertInventory(t *testing.T, available, held int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var gotAvailable, gotHeld, sold int
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT available,held,sold FROM ticket_types WHERE id=$1`, f.ticketTypeID).Scan(&gotAvailable, &gotHeld, &sold))
	require.Equal(t, available, gotAvailable)
	require.Equal(t, held, gotHeld)
	require.Zero(t, sold)
}
func (f *bookingFixture) assertNoBooking(t *testing.T, id uuid.UUID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var bookings, items int
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT count(*) FROM booking WHERE id=$1`, id).Scan(&bookings))
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT count(*) FROM booking_item WHERE booking_id=$1`, id).Scan(&items))
	require.Zero(t, bookings)
	require.Zero(t, items)
}

func TestCreateBookingPostgres(t *testing.T) {
	f := newBookingFixture(t, 5)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	draft := domain.NewBooking(f.UserID, f.sessionID)
	saved, err := f.repo.CreateBooking(ctx, draft, f.eventID, f.ticketTypeID, 2)
	require.NoError(t, err)
	require.Equal(t, draft.ID, saved.ID)
	require.Equal(t, f.UserID, saved.UserID)
	require.Equal(t, f.sessionID, saved.SessionID)
	require.Equal(t, domain.BookingHeld, saved.Status)
	require.WithinDuration(t, draft.ExpiresAt, saved.ExpiresAt, time.Microsecond)
	require.WithinDuration(t, draft.CreatedAt, saved.CreatedAt, time.Microsecond)
	f.assertInventory(t, 3, 2)
	var ticketTypeID uuid.UUID
	var quantity int
	var price, currency string
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT ticket_type_id,quantity,unit_price::text,currency FROM booking_item WHERE booking_id=$1`, saved.ID).
		Scan(&ticketTypeID, &quantity, &price, &currency))
	require.Equal(t, f.ticketTypeID, ticketTypeID)
	require.Equal(t, 2, quantity)
	require.Equal(t, "1500.25", price)
	require.Equal(t, "RUB", currency)
	f.Exec(t, `UPDATE ticket_types SET price=2000 WHERE id=$1`, f.ticketTypeID)
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT unit_price::text FROM booking_item WHERE booking_id=$1`, saved.ID).Scan(&price))
	require.Equal(t, "1500.25", price, "booking keeps the unit price, not the tariff's later price or the total")
	// A duplicate booking ID must not reserve inventory a second time.
	_, err = f.repo.CreateBooking(ctx, draft, f.eventID, f.ticketTypeID, 2)
	require.ErrorIs(t, err, coreerrors.ErrConflict)
	f.assertInventory(t, 3, 2)
}

func TestCreateBookingRejectsInvalidStatePostgres(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*testing.T, *bookingFixture, *domain.Booking, *uuid.UUID, *uuid.UUID, *int)
		want   error
	}{
		{"insufficient inventory", func(t *testing.T, f *bookingFixture, b *domain.Booking, e, tt *uuid.UUID, q *int) { *q = 6 }, coreerrors.ErrConflict},
		{"zero quantity", func(t *testing.T, f *bookingFixture, b *domain.Booking, e, tt *uuid.UUID, q *int) { *q = 0 }, coreerrors.ErrInvalidArgument},
		{"negative quantity", func(t *testing.T, f *bookingFixture, b *domain.Booking, e, tt *uuid.UUID, q *int) { *q = -1 }, coreerrors.ErrInvalidArgument},
		{"draft event", func(t *testing.T, f *bookingFixture, b *domain.Booking, e, tt *uuid.UUID, q *int) {
			f.Exec(t, `UPDATE events SET status='draft' WHERE id=$1`, *e)
		}, coreerrors.ErrNotFound},
		{"archived event", func(t *testing.T, f *bookingFixture, b *domain.Booking, e, tt *uuid.UUID, q *int) {
			f.Exec(t, `UPDATE events SET status='archived' WHERE id=$1`, *e)
		}, coreerrors.ErrNotFound},
		{"draft session", func(t *testing.T, f *bookingFixture, b *domain.Booking, e, tt *uuid.UUID, q *int) {
			f.Exec(t, `UPDATE sessions SET status='draft' WHERE id=$1`, b.SessionID)
		}, coreerrors.ErrNotFound},
		{"cancelled session", func(t *testing.T, f *bookingFixture, b *domain.Booking, e, tt *uuid.UUID, q *int) {
			f.Exec(t, `UPDATE sessions SET status='cancelled' WHERE id=$1`, b.SessionID)
		}, coreerrors.ErrNotFound},
		{"wrong event", func(t *testing.T, f *bookingFixture, b *domain.Booking, e, tt *uuid.UUID, q *int) {
			other := f.Event(t, f.OrgID, "Other")
			f.Exec(t, `UPDATE events SET status='published' WHERE id=$1`, other.ID)
			*e = other.ID
		}, coreerrors.ErrNotFound},
		{"missing tariff", func(t *testing.T, f *bookingFixture, b *domain.Booking, e, tt *uuid.UUID, q *int) { *tt = uuid.New() }, coreerrors.ErrNotFound},
		{"tariff belongs to another session", func(t *testing.T, f *bookingFixture, b *domain.Booking, e, tt *uuid.UUID, q *int) {
			otherSession := f.Session(t, f.eventID)
			f.Exec(t, `UPDATE ticket_types SET session_id=$1 WHERE id=$2`, otherSession, *tt)
		}, coreerrors.ErrNotFound},
		{"sales not started", func(t *testing.T, f *bookingFixture, b *domain.Booking, e, tt *uuid.UUID, q *int) {
			f.Exec(t, `UPDATE sessions SET sales_start_at=$1 WHERE id=$2`, f.now.Add(time.Hour), b.SessionID)
		}, coreerrors.ErrConflict},
		{"sales ended", func(t *testing.T, f *bookingFixture, b *domain.Booking, e, tt *uuid.UUID, q *int) {
			f.Exec(t, `UPDATE sessions SET sales_start_at=$1,sales_end_at=$2 WHERE id=$3`, f.now.Add(-2*time.Hour), f.now.Add(-time.Hour), b.SessionID)
		}, coreerrors.ErrConflict},
		{"session started", func(t *testing.T, f *bookingFixture, b *domain.Booking, e, tt *uuid.UUID, q *int) {
			f.Exec(t, `UPDATE sessions SET starts_at=$1,sales_start_at=$2,sales_end_at=$3 WHERE id=$4`, f.now.Add(-time.Hour), f.now.Add(-3*time.Hour), f.now.Add(-2*time.Hour), b.SessionID)
		}, coreerrors.ErrConflict},
		{"booking expired while waiting", func(t *testing.T, f *bookingFixture, b *domain.Booking, e, tt *uuid.UUID, q *int) {
			b.CreatedAt = f.now.Add(-time.Hour)
			b.ExpiresAt = f.now.Add(-time.Minute)
		}, coreerrors.ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newBookingFixture(t, 5)
			b := domain.NewBooking(f.UserID, f.sessionID)
			event, tariff, quantity := f.eventID, f.ticketTypeID, 2
			tc.change(t, f, &b, &event, &tariff, &quantity)
			_, err := f.repo.CreateBooking(context.Background(), b, event, tariff, quantity)
			require.ErrorIs(t, err, tc.want)
			f.assertInventory(t, 5, 0)
			f.assertNoBooking(t, b.ID)
		})
	}
}

func TestCreateBookingRollbackPostgres(t *testing.T) {
	for _, failItem := range []bool{false, true} {
		name := "booking insert failure"
		if failItem {
			name = "item insert failure"
		}
		t.Run(name, func(t *testing.T) {
			f := newBookingFixture(t, 5)
			b := domain.NewBooking(f.UserID, f.sessionID)
			if !failItem {
				b.UserID = uuid.New() // FK violation after inventory has been reserved.
			} else {
				// Inject a failure only for this booking in the disposable test database.
				// This proves that both the booking INSERT and inventory UPDATE roll back.
				constraint := pgx.Identifier{"test_item_failure_" + strings.ReplaceAll(b.ID.String(), "-", "")}.Sanitize()
				f.Exec(t, `ALTER TABLE booking_item ADD CONSTRAINT `+constraint+` CHECK(booking_id <> '`+b.ID.String()+`'::uuid)`)
				t.Cleanup(func() { f.Exec(t, `ALTER TABLE booking_item DROP CONSTRAINT `+constraint) })
			}
			_, err := f.repo.CreateBooking(context.Background(), b, f.eventID, f.ticketTypeID, 2)
			require.Error(t, err)
			f.assertInventory(t, 5, 0)
			f.assertNoBooking(t, b.ID)
			next := domain.NewBooking(f.UserID, f.sessionID)
			_, err = f.repo.CreateBooking(context.Background(), next, f.eventID, f.ticketTypeID, 1)
			require.NoError(t, err, "failed transaction must release locks and leave the pool usable")
			f.assertInventory(t, 4, 1)
		})
	}
}

func TestCreateBookingConcurrentLastTicketPostgres(t *testing.T) {
	f := newBookingFixture(t, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	const buyers = 12
	start := make(chan struct{})
	results := make(chan error, buyers)
	var wg sync.WaitGroup
	for range buyers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b := domain.NewBooking(f.UserID, f.sessionID)
			<-start
			_, err := f.repo.CreateBooking(ctx, b, f.eventID, f.ticketTypeID, 1)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else {
			require.True(t, errors.Is(err, coreerrors.ErrConflict), "unexpected error: %v", err)
			conflicts++
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, buyers-1, conflicts)
	f.assertInventory(t, 0, 1)
	var bookings, items, quantity int
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT count(*) FROM booking WHERE session_id=$1`, f.sessionID).Scan(&bookings))
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT count(*),coalesce(sum(quantity),0) FROM booking_item WHERE ticket_type_id=$1`, f.ticketTypeID).Scan(&items, &quantity))
	require.Equal(t, 1, bookings)
	require.Equal(t, 1, items)
	require.Equal(t, 1, quantity)
}
