package booking_transport_test

import (
	"context"
	"errors"
	"fmt"
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
	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	service "github.com/poponyas/TicketRush/internal/feature/booking/service"
	transport "github.com/poponyas/TicketRush/internal/feature/booking/transport"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestCreateBookingLogsOneOutcomeWithContext(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		outcome string
		level   zapcore.Level
	}{
		{"success", nil, 201, "success", zap.InfoLevel},
		{"invalid argument", fmt.Errorf("booking: %w", coreerrors.ErrInvalidArgument), 400, "rejected", zap.WarnLevel},
		{"unauthenticated", fmt.Errorf("booking: %w", coreerrors.ErrUnauthenticated), 401, "rejected", zap.WarnLevel},
		{"forbidden", fmt.Errorf("booking: %w", coreerrors.ErrForbidden), 403, "rejected", zap.WarnLevel},
		{"missing resource", fmt.Errorf("booking: %w", coreerrors.ErrNotFound), 404, "rejected", zap.WarnLevel},
		{"conflict", fmt.Errorf("booking: %w", coreerrors.ErrConflict), 409, "rejected", zap.WarnLevel},
		{"payload too large", fmt.Errorf("booking: %w", coreerrors.ErrPayloadTooLarge), 413, "rejected", zap.WarnLevel},
		{"unsupported media type", fmt.Errorf("booking: %w", coreerrors.ErrUnsupportedMediaType), 415, "rejected", zap.WarnLevel},
		{"internal", errors.New("database unavailable"), 500, "error", zap.ErrorLevel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			core, entries := observer.New(zap.DebugLevel)
			log := &corelogger.Logger{Logger: zap.New(core).With(zap.String("request_id", "booking-log-test"))}
			event, session, tariff, user, booking := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
			const serviceDelay = 2 * time.Millisecond
			svc := &bookingServiceStub{create: func(context.Context, service.CreateBookingInput) (domain.Booking, error) {
				time.Sleep(serviceDelay)
				return domain.Booking{ID: booking, SessionID: session, Status: domain.BookingHeld}, tc.err
			}}
			router := chi.NewRouter()
			router.Post("/api/v1/events/{event_id}/sessions/{session_id}/bookings", transport.NewHTTPBookingTransport(svc, transport.NewBookingMetrics()).CreateBooking)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/events/"+event.String()+"/sessions/"+session.String()+"/bookings",
				strings.NewReader(`{"ticket_type_id":"`+tariff.String()+`","quantity":2}`))
			req.Header.Set("Content-Type", "application/json")
			ctx := corelogger.ToContext(req.Context(), log)
			ctx = auth.WithIdentity(ctx, auth.Identity{UserID: user, Roles: []domain.Role{domain.RoleBuyer}})
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req.WithContext(ctx))

			require.Equal(t, tc.status, w.Code)
			records := entries.All()
			require.Len(t, records, 1, "one completed attempt must emit one outcome record")
			entry := records[0]
			require.Equal(t, tc.level, entry.Level)
			fields := entry.ContextMap()
			require.Equal(t, "booking.create", fields["operation"])
			require.Equal(t, "booking-log-test", fields["request_id"])
			require.Equal(t, tc.outcome, fields["outcome"])
			require.Equal(t, event.String(), fields["event_id"])
			require.Equal(t, session.String(), fields["session_id"])
			require.Equal(t, tariff.String(), fields["ticket_type_id"])
			require.EqualValues(t, 2, fields["quantity"])
			duration, ok := fields["duration_ms"].(float64)
			require.True(t, ok, "duration_ms must be numeric milliseconds")
			require.GreaterOrEqual(t, duration, float64(serviceDelay/time.Millisecond))
			seen := make(map[string]bool)
			for _, field := range entry.Context {
				require.False(t, seen[field.Key], "duplicate field %q", field.Key)
				seen[field.Key] = true
			}
			if tc.err == nil {
				require.Equal(t, "booking created", entry.Message)
				require.Equal(t, booking.String(), fields["booking_id"])
				require.NotContains(t, fields, "error")
			} else {
				require.Equal(t, "create booking", entry.Message)
				require.Equal(t, tc.err.Error(), fields["error"])
				require.NotContains(t, fields, "booking_id")
			}
		})
	}
}

func TestCreateBookingInvalidJSONDoesNotLogServiceOutcome(t *testing.T) {
	core, entries := observer.New(zap.DebugLevel)
	log := &corelogger.Logger{Logger: zap.New(core)}
	svc := &bookingServiceStub{create: func(context.Context, service.CreateBookingInput) (domain.Booking, error) {
		t.Fatal("invalid JSON must not reach the service")
		return domain.Booking{}, nil
	}}
	router := chi.NewRouter()
	router.Post("/api/v1/events/{event_id}/sessions/{session_id}/bookings", transport.NewHTTPBookingTransport(svc, transport.NewBookingMetrics()).CreateBooking)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/events/"+uuid.NewString()+"/sessions/"+uuid.NewString()+"/bookings", strings.NewReader(`{"quantity":`))
	req.Header.Set("Content-Type", "application/json")
	ctx := auth.WithIdentity(corelogger.ToContext(req.Context(), log), auth.Identity{UserID: uuid.New(), Roles: []domain.Role{domain.RoleBuyer}})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req.WithContext(ctx))
	require.Equal(t, http.StatusBadRequest, w.Code)
	records := entries.All()
	require.Len(t, records, 1)
	require.Equal(t, zap.WarnLevel, records[0].Level)
	fields := records[0].ContextMap()
	require.Equal(t, "booking.create", fields["operation"])
	require.NotContains(t, fields, "duration_ms")
	require.NotContains(t, fields, "outcome")
}
