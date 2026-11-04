package booking_repository_test

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/booking/service"
	transport "github.com/poponyas/TicketRush/internal/feature/booking/transport"
	"github.com/stretchr/testify/require"
)

func TestGetBookingPostgres(t *testing.T) {
	f := newBookingFixture(t, 5)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	created, err := f.repo.CreateBooking(ctx, domain.NewBooking(f.UserID, f.sessionID), f.eventID, f.ticketTypeID, 2)
	require.NoError(t, err)
	f.Exec(t, `UPDATE ticket_types SET price=2000 WHERE id=$1`, f.ticketTypeID)
	got, items, err := f.repo.GetBooking(ctx, created.ID, f.UserID)
	require.NoError(t, err)
	require.Equal(t, created, got)
	require.Len(t, items, 1)
	require.Equal(t, created.ID, items[0].BookingID)
	require.Equal(t, f.ticketTypeID, items[0].TicketTypeID)
	require.Equal(t, 2, items[0].Quantity)
	require.Equal(t, "1500.25", items[0].UnitPrice.Amount.String())
	require.Equal(t, "RUB", items[0].UnitPrice.Currency)
	for _, status := range []domain.BookingStatus{domain.BookingHeld, domain.BookingConfirmed, domain.BookingCancelled, domain.BookingExpired} {
		f.Exec(t, `UPDATE booking SET status=$1 WHERE id=$2`, status, created.ID)
		got, items, err = f.repo.GetBooking(ctx, created.ID, f.UserID)
		require.NoError(t, err)
		require.Equal(t, status, got.Status)
		require.Len(t, items, 1)
	}
	for _, ids := range [][2]uuid.UUID{{created.ID, f.OtherUserID}, {uuid.New(), f.UserID}} {
		_, _, err := f.repo.GetBooking(ctx, ids[0], ids[1])
		require.ErrorIs(t, err, coreerrors.ErrNotFound)
	}
	// The schema permits a booking without items. LEFT JOIN still returns it.
	empty := domain.NewBooking(f.UserID, f.sessionID)
	f.Exec(t, `INSERT INTO booking(id,user_id,session_id,status,expires_at,created_at) VALUES($1,$2,$3,$4,$5,$6)`,
		empty.ID, empty.UserID, empty.SessionID, empty.Status, empty.ExpiresAt, empty.CreatedAt)
	_, emptyItems, err := f.repo.GetBooking(ctx, empty.ID, f.UserID)
	require.NoError(t, err)
	require.NotNil(t, emptyItems)
	require.Empty(t, emptyItems)
	// Do not collapse several positions into one, even though POST currently accepts one tariff.
	second := uuid.New()
	f.Exec(t, `INSERT INTO ticket_types(id,session_id,name,price,currency,capacity,available,held,sold)
  VALUES($1,$2,'Second',100,'RUB',1,0,1,0)`, second, f.sessionID)
	f.Exec(t, `INSERT INTO booking_item(id,booking_id,ticket_type_id,quantity,unit_price,currency)
  VALUES($1,$2,$3,1,100,'RUB')`, uuid.New(), created.ID, second)
	_, items, err = f.repo.GetBooking(ctx, created.ID, f.UserID)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.NotEqual(t, items[0].TicketTypeID, items[1].TicketTypeID)
	require.Less(t, items[0].ID.String(), items[1].ID.String())
}

func TestCreateAndGetOwnBookingHTTPPostgres(t *testing.T) {
	f := newBookingFixture(t, 5)
	h := transport.NewHTTPBookingTransport(service.NewBookingService(f.repo), transport.NewBookingMetrics())
	router := chi.NewRouter()
	router.Post("/api/v1/events/{event_id}/sessions/{session_id}/bookings", h.CreateBooking)
	router.Get("/api/v1/bookings/{booking_id}", h.GetBooking)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/events/"+f.eventID.String()+"/sessions/"+f.sessionID.String()+"/bookings",
		strings.NewReader(`{"ticket_type_id":"`+f.ticketTypeID.String()+`","quantity":2}`))
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(auth.WithIdentity(r.Context(), auth.Identity{UserID: f.UserID, Roles: []domain.Role{domain.RoleBuyer}}))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created transport.BookingResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.NotEqual(t, uuid.Nil, created.ID)
	f.assertInventory(t, 3, 2)
	for _, tc := range []struct {
		name     string
		id, user uuid.UUID
		code     int
	}{
		{"owner", created.ID, f.UserID, 200},
		{"other buyer", created.ID, f.OtherUserID, 404},
		{"missing", uuid.New(), f.UserID, 404},
		{"anonymous", created.ID, uuid.Nil, 401},
		{"zero booking ID", uuid.Nil, f.UserID, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/bookings/"+tc.id.String()+"?user_id="+f.UserID.String(), nil)
			if tc.user != uuid.Nil {
				r = r.WithContext(auth.WithIdentity(r.Context(), auth.Identity{UserID: tc.user, Roles: []domain.Role{domain.RoleBuyer}}))
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			require.Equal(t, tc.code, w.Code, w.Body.String())
			if tc.code == 200 {
				var body transport.BookingGetResponse
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				require.Equal(t, created.ID, body.ID)
				require.Equal(t, f.sessionID, body.SessionID)
				require.Len(t, body.Items, 1)
				require.Equal(t, 2, body.Items[0].Quantity)
				require.Equal(t, "1500.25", body.Items[0].UnitPrice.Amount.String())
				require.Equal(t, "RUB", body.Items[0].UnitPrice.Currency)
			} else {
				require.NotContains(t, w.Body.String(), created.ID.String())
				require.NotContains(t, w.Body.String(), "1500.25")
			}
		})
	}
}
