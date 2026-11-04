package booking_repository_test

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/booking/service"
	transport "github.com/poponyas/TicketRush/internal/feature/booking/transport"
	"github.com/stretchr/testify/require"
)

func heldBooking(t *testing.T, f *bookingFixture, quantity int) domain.Booking {
	t.Helper()
	b, err := f.repo.CreateBooking(context.Background(), domain.NewBooking(f.UserID, f.sessionID), f.eventID, f.ticketTypeID, quantity)
	require.NoError(t, err)
	return b
}
func assertBookingStatus(t *testing.T, f *bookingFixture, id uuid.UUID, want domain.BookingStatus) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var got domain.BookingStatus
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT status FROM booking WHERE id=$1`, id).Scan(&got))
	require.Equal(t, want, got)
}

func TestCancelBookingPostgres(t *testing.T) {
	f := newBookingFixture(t, 5)
	b := heldBooking(t, f, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	saved, err := f.repo.CancelBooking(ctx, b.ID, f.UserID)
	require.NoError(t, err)
	require.Equal(t, b.ID, saved.ID)
	require.Equal(t, b.UserID, saved.UserID)
	require.Equal(t, b.SessionID, saved.SessionID)
	require.Equal(t, b.ExpiresAt, saved.ExpiresAt)
	require.Equal(t, domain.BookingCancelled, saved.Status)
	f.assertInventory(t, 5, 0)
	assertBookingStatus(t, f, b.ID, domain.BookingCancelled)
	_, items, err := f.repo.GetBooking(ctx, b.ID, f.UserID)
	require.NoError(t, err)
	require.Len(t, items, 1, "cancellation preserves booking positions")
	// Repetition remains successful even after the old reservation deadline passes.
	f.Exec(t, `UPDATE booking SET created_at=now()-interval '2 hours',expires_at=now()-interval '1 hour' WHERE id=$1`, b.ID)
	saved, err = f.repo.CancelBooking(ctx, b.ID, f.UserID)
	require.NoError(t, err)
	require.Equal(t, domain.BookingCancelled, saved.Status)
	f.assertInventory(t, 5, 0)
}

func TestCancelBookingRejectedPostgres(t *testing.T) {
	for _, name := range []string{"foreign", "missing", "confirmed", "expired", "held past deadline"} {
		t.Run(name, func(t *testing.T) {
			f := newBookingFixture(t, 5)
			b := heldBooking(t, f, 2)
			id, user := b.ID, f.UserID
			want := coreerrors.ErrConflict
			status := domain.BookingHeld
			switch name {
			case "foreign":
				user = f.OtherUserID
				want = coreerrors.ErrNotFound
			case "missing":
				id = uuid.New()
				want = coreerrors.ErrNotFound
			case "confirmed":
				status = domain.BookingConfirmed
				f.Exec(t, `UPDATE booking SET status='confirmed' WHERE id=$1`, id)
			case "expired":
				status = domain.BookingExpired
				f.Exec(t, `UPDATE booking SET status='expired' WHERE id=$1`, id)
			case "held past deadline":
				f.Exec(t, `UPDATE booking SET created_at=now()-interval '2 hours',expires_at=now()-interval '1 hour' WHERE id=$1`, id)
			}
			_, err := f.repo.CancelBooking(context.Background(), id, user)
			require.ErrorIs(t, err, want)
			f.assertInventory(t, 3, 2)
			assertBookingStatus(t, f, b.ID, status)
		})
	}
}

func TestCancelBookingMultipleItemsPostgres(t *testing.T) {
	f := newBookingFixture(t, 5)
	b := heldBooking(t, f, 2)
	// Two positions share a tariff; one more uses a different tariff.
	f.Exec(t, `INSERT INTO booking_item(id,booking_id,ticket_type_id,quantity,unit_price,currency) VALUES($1,$2,$3,1,1500.25,'RUB')`, uuid.New(), b.ID, f.ticketTypeID)
	f.Exec(t, `UPDATE ticket_types SET available=2,held=3 WHERE id=$1`, f.ticketTypeID)
	second := uuid.New()
	f.Exec(t, `INSERT INTO ticket_types(id,session_id,name,price,currency,capacity,available,held,sold) VALUES($1,$2,'Second',100,'RUB',1,0,1,0)`, second, f.sessionID)
	f.Exec(t, `INSERT INTO booking_item(id,booking_id,ticket_type_id,quantity,unit_price,currency) VALUES($1,$2,$3,1,100,'RUB')`, uuid.New(), b.ID, second)
	_, err := f.repo.CancelBooking(context.Background(), b.ID, f.UserID)
	require.NoError(t, err)
	f.assertInventory(t, 5, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var available, held int
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT available,held FROM ticket_types WHERE id=$1`, second).Scan(&available, &held))
	require.Equal(t, 1, available)
	require.Zero(t, held)
}

func TestCancelBookingRollbackPostgres(t *testing.T) {
	for _, name := range []string{"inventory mismatch", "status update failure"} {
		t.Run(name, func(t *testing.T) {
			f := newBookingFixture(t, 5)
			b := heldBooking(t, f, 2)
			if name == "inventory mismatch" {
				// The first tariff is released before a mismatch in the final tariff.
				second := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
				f.Exec(t, `INSERT INTO ticket_types(id,session_id,name,price,currency,capacity,available,held,sold) VALUES($1,$2,'Broken fixture',100,'RUB',1,1,0,0)`, second, f.sessionID)
				f.Exec(t, `INSERT INTO booking_item(id,booking_id,ticket_type_id,quantity,unit_price,currency) VALUES($1,$2,$3,1,100,'RUB')`, uuid.New(), b.ID, second)
			} else {
				constraint := pgx.Identifier{"test_cancel_failure_" + strings.ReplaceAll(b.ID.String(), "-", "")}.Sanitize()
				f.Exec(t, `ALTER TABLE booking ADD CONSTRAINT `+constraint+` CHECK(id <> '`+b.ID.String()+`'::uuid OR status <> 'cancelled')`)
				t.Cleanup(func() { f.Exec(t, `ALTER TABLE booking DROP CONSTRAINT `+constraint) })
			}
			_, err := f.repo.CancelBooking(context.Background(), b.ID, f.UserID)
			require.Error(t, err)
			if name == "inventory mismatch" {
				require.ErrorIs(t, err, coreerrors.ErrConflict)
			}
			f.assertInventory(t, 3, 2)
			assertBookingStatus(t, f, b.ID, domain.BookingHeld)
		})
	}
}

func TestCancelBookingConcurrentPostgres(t *testing.T) {
	f := newBookingFixture(t, 1)
	b := heldBooking(t, f, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	const callers = 12
	start := make(chan struct{})
	type result struct {
		booking domain.Booking
		err     error
	}
	results := make(chan result, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			saved, err := f.repo.CancelBooking(ctx, b.ID, f.UserID)
			results <- result{saved, err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for result := range results {
		require.NoError(t, result.err)
		require.Equal(t, b.ID, result.booking.ID)
		require.Equal(t, domain.BookingCancelled, result.booking.Status)
	}
	f.assertInventory(t, 1, 0)
	assertBookingStatus(t, f, b.ID, domain.BookingCancelled)
}

func TestCancelOwnBookingHTTPPostgres(t *testing.T) {
	f := newBookingFixture(t, 5)
	b := heldBooking(t, f, 2)
	router := chi.NewRouter()
	router.Post("/api/v1/bookings/{booking_id}/cancel", transport.NewHTTPBookingTransport(service.NewBookingService(f.repo), transport.NewBookingMetrics()).CancelBooking)
	for _, tc := range []struct {
		name string
		user uuid.UUID
		code int
	}{
		{"anonymous", uuid.Nil, 401}, {"other buyer", f.OtherUserID, 404},
		{"owner", f.UserID, 200}, {"repeat", f.UserID, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/v1/bookings/"+b.ID.String()+"/cancel?user_id="+f.UserID.String(), nil)
			if tc.user != uuid.Nil {
				r = r.WithContext(auth.WithIdentity(r.Context(), auth.Identity{UserID: tc.user, Roles: []domain.Role{domain.RoleBuyer}}))
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			require.Equal(t, tc.code, w.Code, w.Body.String())
			if tc.code == 200 {
				var body transport.BookingResponse
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				require.Equal(t, b.ID, body.ID)
				require.Equal(t, domain.BookingCancelled, body.Status)
				f.assertInventory(t, 5, 0)
			} else {
				f.assertInventory(t, 3, 2)
			}
		})
	}
}
