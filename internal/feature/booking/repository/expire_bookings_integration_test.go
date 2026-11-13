package booking_repository_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	booking_events "github.com/poponyas/TicketRush/internal/feature/booking/events"
	repository "github.com/poponyas/TicketRush/internal/feature/booking/repository"
	"github.com/stretchr/testify/require"
)

// Expiration intentionally has no user scope. Isolate the tables it changes so
// the global scan cannot expire fixtures belonging to another test/package.
func newExpirationFixture(t *testing.T, capacity int) *bookingFixture {
	t.Helper()
	f := newBookingFixture(t, capacity)
	original := f.Pool
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	schema := pgx.Identifier{"expiry_test_" + uuid.New().String()}.Sanitize()
	_, err := original.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := original.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		if err != nil {
			t.Errorf("cleanup expiration schema: %v", err)
		}
	})
	// Clone migrated columns, checks and indexes. Parent entities are read from
	// public; FK behavior is covered by the non-isolated booking integration tests.
	for _, table := range []string{"booking", "booking_item", "ticket_types", "outbox_events", "tickets"} {
		name := pgx.Identifier{table}.Sanitize()
		_, err := original.Exec(ctx, "CREATE TABLE "+schema+"."+name+" (LIKE "+name+" INCLUDING ALL)")
		require.NoError(t, err)
	}
	_, err = original.Exec(ctx, "INSERT INTO "+schema+".ticket_types SELECT * FROM ticket_types WHERE id=$1", f.ticketTypeID)
	require.NoError(t, err)
	config := original.Config()
	config.ConnConfig.RuntimeParams["search_path"] = schema + ", public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	f.Fixture.Pool = pool
	f.repo = repository.NewBookingRepository(&postgres.Pool{Pool: pool, OpTimeout: 5 * time.Second})
	return f
}
func markOverdue(t *testing.T, f *bookingFixture, b domain.Booking) {
	t.Helper()
	f.Exec(t, `UPDATE booking SET created_at=now()-interval '2 hours',expires_at=now()-interval '1 hour' WHERE id=$1`, b.ID)
}

func TestExpireBookingsPostgres(t *testing.T) {
	f := newExpirationFixture(t, 10)
	a := heldBooking(t, f, 2)
	b := heldBooking(t, f, 1)
	future := heldBooking(t, f, 1)
	markOverdue(t, f, a)
	markOverdue(t, f, b)
	for _, status := range []domain.BookingStatus{domain.BookingConfirmed, domain.BookingCancelled, domain.BookingExpired} {
		f.Exec(t, `INSERT INTO booking(id,user_id,session_id,status,created_at,expires_at)
   VALUES($1,$2,$3,$4,now()-interval '2 hours',now()-interval '1 hour')`, uuid.New(), f.UserID, f.sessionID, status)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	count, err := f.repo.ExpireBookings(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	count, err = f.repo.ExpireBookings(ctx, 100)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	f.assertInventory(t, 9, 1)
	assertBookingStatus(t, f, a.ID, domain.BookingExpired)
	assertBookingStatus(t, f, b.ID, domain.BookingExpired)
	assertBookingStatus(t, f, future.ID, domain.BookingHeld)
	count, err = f.repo.ExpireBookings(ctx, 100)
	require.NoError(t, err)
	require.Zero(t, count)
	f.assertInventory(t, 9, 1)
	var itemCount int
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT count(*) FROM booking_item WHERE booking_id=ANY($1::uuid[])`, []uuid.UUID{a.ID, b.ID}).Scan(&itemCount))
	require.Equal(t, 2, itemCount, "expiration preserves positions and captured prices")
}

func TestExpireBookingsConcurrentSkipLockedPostgres(t *testing.T) {
	const total = 12
	f := newExpirationFixture(t, total)
	bookings := make([]domain.Booking, 0, total)
	for range total {
		b := heldBooking(t, f, 1)
		markOverdue(t, f, b)
		bookings = append(bookings, b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	locker, err := f.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = locker.Rollback(context.Background()) }()
	var id uuid.UUID
	require.NoError(t, locker.QueryRow(ctx, `SELECT id FROM booking WHERE id=$1 FOR UPDATE`, bookings[0].ID).Scan(&id))
	type result struct {
		count int
		err   error
	}
	results := make(chan result, 4)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			processed := 0
			for {
				count, err := f.repo.ExpireBookings(ctx, 2)
				if err != nil {
					results <- result{processed, err}
					return
				}
				processed += count
				if count == 0 {
					results <- result{processed, nil}
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	processed := 0
	for result := range results {
		require.NoError(t, result.err)
		processed += result.count
	}
	require.Equal(t, total-1, processed, "locked booking must be skipped, other bookings processed exactly once")
	f.assertInventory(t, total-1, 1)
	assertBookingStatus(t, f, bookings[0].ID, domain.BookingHeld)
	require.NoError(t, locker.Rollback(ctx))
	count, err := f.repo.ExpireBookings(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	f.assertInventory(t, total, 0)
	count, err = f.repo.ExpireBookings(ctx, 2)
	require.NoError(t, err)
	require.Zero(t, count)
	for _, b := range bookings {
		assertBookingOutbox(t, f, b.ID, booking_events.BookingExpiredType, map[uuid.UUID]int{f.ticketTypeID: 1})
	}
}

func TestExpireBookingsRollbackPostgres(t *testing.T) {
	f := newExpirationFixture(t, 5)
	a := heldBooking(t, f, 2)
	b := heldBooking(t, f, 1)
	markOverdue(t, f, a)
	markOverdue(t, f, b)
	constraint := pgx.Identifier{"test_expiration_failure_" + strings.ReplaceAll(a.ID.String(), "-", "")}.Sanitize()
	f.Exec(t, `ALTER TABLE booking ADD CONSTRAINT `+constraint+` CHECK(id <> '`+a.ID.String()+`'::uuid OR status <> 'expired')`)
	t.Cleanup(func() { f.Exec(t, `ALTER TABLE booking DROP CONSTRAINT IF EXISTS `+constraint) })
	count, err := f.repo.ExpireBookings(context.Background(), 100)
	require.Error(t, err)
	require.Zero(t, count)
	f.assertInventory(t, 2, 3)
	assertBookingStatus(t, f, a.ID, domain.BookingHeld)
	assertBookingStatus(t, f, b.ID, domain.BookingHeld)
	f.Exec(t, `ALTER TABLE booking DROP CONSTRAINT `+constraint)
	count, err = f.repo.ExpireBookings(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	f.assertInventory(t, 5, 0)
}

func TestExpireBookingsAndCancellationPostgres(t *testing.T) {
	f := newExpirationFixture(t, 1)
	b := heldBooking(t, f, 1)
	markOverdue(t, f, b)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	type result struct {
		count int
		err   error
	}
	expired := make(chan result, 1)
	cancelled := make(chan error, 1)
	start := make(chan struct{})
	go func() { <-start; count, err := f.repo.ExpireBookings(ctx, 1); expired <- result{count, err} }()
	go func() { <-start; _, err := f.repo.CancelBooking(ctx, b.ID, f.UserID); cancelled <- err }()
	close(start)
	require.ErrorIs(t, <-cancelled, coreerrors.ErrConflict)
	outcome := <-expired
	require.NoError(t, outcome.err)
	require.LessOrEqual(t, outcome.count, 1)
	// The worker may skip the row while cancellation owns its lock. A later poll
	// must pick it up, without ever releasing the ticket twice.
	count, err := f.repo.ExpireBookings(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, outcome.count+count)
	f.assertInventory(t, 1, 0)
	assertBookingStatus(t, f, b.ID, domain.BookingExpired)
	assertBookingOutbox(t, f, b.ID, booking_events.BookingExpiredType, map[uuid.UUID]int{f.ticketTypeID: 1})
	require.Empty(t, readBookingOutbox(t, f, b.ID, booking_events.BookingCancelledType))
}

func TestExpireBookingsRejectsInvalidBatch(t *testing.T) {
	for _, limit := range []int{-1, 0, 1001} {
		_, err := (&repository.BookingRepository{}).ExpireBookings(context.Background(), limit)
		require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	}
}
