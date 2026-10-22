package booking_transport_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
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

type cancellationService struct {
	transport.BookingService
	cancel func(context.Context, uuid.UUID, uuid.UUID) (domain.Booking, error)
}

func (s *cancellationService) CancelBooking(ctx context.Context, id, user uuid.UUID) (domain.Booking, error) {
	return s.cancel(ctx, id, user)
}

func TestCancelBookingHTTP(t *testing.T) {
	id, user := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name string
		err  error
		code int
	}{
		{"buyer success", nil, 200}, {"foreign or missing", coreerrors.ErrNotFound, 404},
		{"conflict", coreerrors.ErrConflict, 409}, {"database", errors.New("postgres password=secret"), 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &cancellationService{cancel: func(ctx context.Context, b, u uuid.UUID) (domain.Booking, error) {
				require.Equal(t, id, b)
				require.Equal(t, user, u)
				return domain.Booking{ID: id, SessionID: uuid.New(), Status: domain.BookingCancelled}, tc.err
			}}
			router := chi.NewRouter()
			router.Post("/api/v1/bookings/{booking_id}/cancel", transport.NewHTTPBookingTransport(svc).CancelBooking)
			r := httptest.NewRequest(http.MethodPost, "/api/v1/bookings/"+id.String()+"/cancel?user_id="+uuid.New().String(), nil)
			r = r.WithContext(auth.WithIdentity(r.Context(), auth.Identity{UserID: user, Roles: []domain.Role{domain.RoleBuyer}}))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			require.Equal(t, tc.code, w.Code)
			require.NotContains(t, w.Body.String(), "secret")
			if tc.err == nil {
				var body transport.BookingResponse
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				require.Equal(t, id, body.ID)
				require.Equal(t, domain.BookingCancelled, body.Status)
			}
		})
	}
}

func TestCancelBookingHTTPAuthenticationAndIDs(t *testing.T) {
	svc := &cancellationService{cancel: func(context.Context, uuid.UUID, uuid.UUID) (domain.Booking, error) {
		t.Fatal("invalid request must not reach the service")
		return domain.Booking{}, nil
	}}
	for _, tc := range []struct {
		id       string
		identity *auth.Identity
		code     int
	}{
		{uuid.New().String(), nil, 401}, {uuid.New().String(), &auth.Identity{}, 401},
		{"not-a-uuid", &auth.Identity{UserID: uuid.New()}, 400},
	} {
		router := chi.NewRouter()
		router.Post("/api/v1/bookings/{booking_id}/cancel", transport.NewHTTPBookingTransport(svc).CancelBooking)
		r := httptest.NewRequest(http.MethodPost, "/api/v1/bookings/"+tc.id+"/cancel", nil)
		if tc.identity != nil {
			r = r.WithContext(auth.WithIdentity(r.Context(), *tc.identity))
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		require.Equal(t, tc.code, w.Code)
	}
	router := chi.NewRouter()
	router.Post("/api/v1/bookings/{booking_id}/cancel", transport.NewHTTPBookingTransport(service.NewBookingService(nil)).CancelBooking)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/bookings/"+uuid.Nil.String()+"/cancel", nil)
	r = r.WithContext(auth.WithIdentity(r.Context(), auth.Identity{UserID: uuid.New()}))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	require.Equal(t, 400, w.Code)
}
