package booking_transport_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/booking/service"
	transport "github.com/poponyas/TicketRush/internal/feature/booking/transport"
	"github.com/stretchr/testify/require"
)

type bookingServiceStub struct {
	transport.BookingService
	get    func(context.Context, uuid.UUID, uuid.UUID) (domain.Booking, []domain.BookingItem, error)
	create func(context.Context, service.CreateBookingInput) (domain.Booking, error)
}

func (s *bookingServiceStub) GetBooking(ctx context.Context, id, user uuid.UUID) (domain.Booking, []domain.BookingItem, error) {
	return s.get(ctx, id, user)
}
func (s *bookingServiceStub) CreateBooking(ctx context.Context, input service.CreateBookingInput) (domain.Booking, error) {
	return s.create(ctx, input)
}

func TestGetBookingHTTP(t *testing.T) {
	id, user, tariff := uuid.New(), uuid.New(), uuid.New()
	price, err := domain.NewMoney("1500.25", "RUB")
	require.NoError(t, err)
	for _, tc := range []struct {
		name string
		err  error
		code int
	}{
		{"buyer", nil, 200}, {"foreign or missing", fmt.Errorf("repository: %w", coreerrors.ErrNotFound), 404},
		{"internal", errors.New("postgres password=secret"), 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &bookingServiceStub{get: func(ctx context.Context, b, u uuid.UUID) (domain.Booking, []domain.BookingItem, error) {
				require.Equal(t, id, b)
				require.Equal(t, user, u)
				return domain.Booking{ID: id, SessionID: uuid.New(), Status: domain.BookingHeld}, []domain.BookingItem{{TicketTypeID: tariff, Quantity: 2, UnitPrice: price}}, tc.err
			}}
			router := chi.NewRouter()
			router.Get("/api/v1/bookings/{booking_id}", transport.NewHTTPBookingTransport(svc, transport.NewBookingMetrics()).GetBooking)
			r := httptest.NewRequest(http.MethodGet, "/api/v1/bookings/"+id.String()+"?user_id="+uuid.New().String(), nil)
			r = r.WithContext(auth.WithIdentity(r.Context(), auth.Identity{UserID: user, Roles: []domain.Role{domain.RoleBuyer}}))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			require.Equal(t, tc.code, w.Code)
			require.NotContains(t, w.Body.String(), "secret")
			if tc.err == nil {
				var body transport.BookingGetResponse
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				require.Equal(t, id, body.ID)
				require.Len(t, body.Items, 1)
				require.Equal(t, tariff, body.Items[0].TicketTypeID)
				require.Equal(t, "1500.25", body.Items[0].UnitPrice.Amount.String())
				require.NotContains(t, w.Body.String(), "UserID")
			}
		})
	}
}
func TestGetBookingHTTPAuthenticationAndIDs(t *testing.T) {
	svc := &bookingServiceStub{get: func(context.Context, uuid.UUID, uuid.UUID) (domain.Booking, []domain.BookingItem, error) {
		t.Fatal("invalid request must not reach the service")
		return domain.Booking{}, nil, nil
	}}
	for _, tc := range []struct {
		path     string
		identity *auth.Identity
		code     int
	}{
		{uuid.New().String(), nil, 401},
		{uuid.New().String(), &auth.Identity{}, 401},
		{"bad-uuid", &auth.Identity{UserID: uuid.New()}, 400},
	} {
		router := chi.NewRouter()
		router.Get("/api/v1/bookings/{booking_id}", transport.NewHTTPBookingTransport(svc, transport.NewBookingMetrics()).GetBooking)
		r := httptest.NewRequest(http.MethodGet, "/api/v1/bookings/"+tc.path, nil)
		if tc.identity != nil {
			r = r.WithContext(auth.WithIdentity(r.Context(), *tc.identity))
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		require.Equal(t, tc.code, w.Code)
	}
}
func TestCreateBookingHTTPBuyerAndErrors(t *testing.T) {
	event, session, tariff, user := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, tc := range []struct {
		name, contentType, body string
		serviceErr              error
		code, calls             int
	}{
		{"buyer success", "application/json", `{"ticket_type_id":"` + tariff.String() + `","quantity":2}`, nil, 201, 1},
		{"malformed", "application/json", `{"quantity":`, nil, 400, 0},
		{"unknown field", "application/json", `{"user_id":"spoof"}`, nil, 400, 0},
		{"wrong content type", "text/plain", `{}`, nil, 415, 0},
		{"conflict", "application/json", `{"ticket_type_id":"` + tariff.String() + `","quantity":2}`, coreerrors.ErrConflict, 409, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			svc := &bookingServiceStub{create: func(ctx context.Context, input service.CreateBookingInput) (domain.Booking, error) {
				calls++
				require.Equal(t, event, input.EventID)
				require.Equal(t, session, input.SessionID)
				require.Equal(t, tariff, input.TicketTypeID)
				require.Equal(t, user, input.UserID)
				require.Equal(t, 2, input.Quantity)
				return domain.Booking{ID: uuid.New(), SessionID: session, Status: domain.BookingHeld}, tc.serviceErr
			}}
			router := chi.NewRouter()
			router.Post("/api/v1/events/{event_id}/sessions/{session_id}/bookings", transport.NewHTTPBookingTransport(svc, transport.NewBookingMetrics()).CreateBooking)
			r := httptest.NewRequest(http.MethodPost, "/api/v1/events/"+event.String()+"/sessions/"+session.String()+"/bookings", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.contentType)
			r = r.WithContext(auth.WithIdentity(r.Context(), auth.Identity{UserID: user, Roles: []domain.Role{domain.RoleBuyer}}))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			require.Equal(t, tc.code, w.Code)
			require.Equal(t, tc.calls, calls)
			if tc.code != 201 {
				require.NotContains(t, w.Body.String(), `"session_id"`)
			}
		})
	}
}
