package booking_repository_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	bookingevents "github.com/poponyas/TicketRush/internal/feature/booking/events"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

func confirmedCounts(t *testing.T, f *bookingFixture, id uuid.UUID) (int, int, int) {
	t.Helper()
	var available, held, sold, tickets int
	require.NoError(t, f.Pool.QueryRow(context.Background(), `SELECT available,held,sold FROM ticket_types WHERE id=$1`, f.ticketTypeID).Scan(&available, &held, &sold))
	require.NoError(t, f.Pool.QueryRow(context.Background(), `SELECT count(*) FROM tickets t JOIN booking_item i ON i.id=t.booking_item_id WHERE i.booking_id=$1`, id).Scan(&tickets))
	require.Equal(t, available+held+sold, 5)
	return held, sold, tickets
}
func TestConfirmBookingConcurrentPostgres(t *testing.T) {
	f := newExpirationFixture(t, 5)
	b := heldBooking(t, f, 2)
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.repo.ConfirmBooking(context.Background(), b.ID, f.UserID)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	held, sold, tickets := confirmedCounts(t, f, b.ID)
	require.Zero(t, held)
	require.Equal(t, 2, sold)
	require.Equal(t, 2, tickets)
	assertBookingStatus(t, f, b.ID, domain.BookingConfirmed)
	assertBookingOutbox(t, f, b.ID, bookingevents.BookingConfirmedType, map[uuid.UUID]int{f.ticketTypeID: 2})
	count, err := f.repo.ExpireBookings(context.Background(), 10)
	require.NoError(t, err)
	require.Zero(t, count)
	_, err = f.repo.CancelBooking(context.Background(), b.ID, f.UserID)
	require.ErrorIs(t, err, coreerrors.ErrConflict)
}
func TestConfirmBookingFailuresPostgres(t *testing.T) {
	for _, mode := range []string{"foreign user", "expired", "cancelled session", "outbox failure"} {
		t.Run(mode, func(t *testing.T) {
			f := newExpirationFixture(t, 5)
			b := heldBooking(t, f, 2)
			user := f.UserID
			want := coreerrors.ErrConflict
			switch mode {
			case "foreign user":
				user = f.OtherUserID
				want = coreerrors.ErrNotFound
			case "expired":
				markOverdue(t, f, b)
			case "cancelled session":
				f.Exec(t, `UPDATE sessions SET status='cancelled' WHERE id=$1`, f.sessionID)
			case "outbox failure":
				rejectBookingOutbox(t, f, b.ID, bookingevents.BookingConfirmedType)
			}
			_, err := f.repo.ConfirmBooking(context.Background(), b.ID, user)
			require.Error(t, err)
			if mode != "outbox failure" {
				require.ErrorIs(t, err, want)
			}
			held, sold, tickets := confirmedCounts(t, f, b.ID)
			require.Equal(t, 2, held)
			require.Zero(t, sold)
			require.Zero(t, tickets)
			assertBookingStatus(t, f, b.ID, domain.BookingHeld)
			require.Empty(t, readBookingOutbox(t, f, b.ID, bookingevents.BookingConfirmedType))
		})
	}
}
func TestConfirmBookingCancellationRacePostgres(t *testing.T) {
	f := newExpirationFixture(t, 5)
	b := heldBooking(t, f, 2)
	errs := make(chan error, 2)
	start := make(chan struct{})
	go func() { <-start; _, err := f.repo.ConfirmBooking(context.Background(), b.ID, f.UserID); errs <- err }()
	go func() { <-start; _, err := f.repo.CancelBooking(context.Background(), b.ID, f.UserID); errs <- err }()
	close(start)
	a, c := <-errs, <-errs
	require.True(t, (a == nil && errors.Is(c, coreerrors.ErrConflict)) || (c == nil && errors.Is(a, coreerrors.ErrConflict)))
	held, sold, tickets := confirmedCounts(t, f, b.ID)
	require.Zero(t, held)
	require.Equal(t, sold, tickets)
	require.True(t, sold == 0 || sold == 2)
}
