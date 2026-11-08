package analytics_transport_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	"github.com/poponyas/TicketRush/internal/feature/analytics"
	service "github.com/poponyas/TicketRush/internal/feature/analytics/service"
	transport "github.com/poponyas/TicketRush/internal/feature/analytics/transport"
	booking_events "github.com/poponyas/TicketRush/internal/feature/booking/events"
	"github.com/stretchr/testify/require"
)

type eventRepository struct {
	get func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Event, error)
}

func (r eventRepository) GetEvent(ctx context.Context, event, org, user uuid.UUID) (domain.Event, error) {
	return r.get(ctx, event, org, user)
}

type statsRepository struct {
	get func(context.Context, uuid.UUID, time.Time, time.Time) ([]analytics.BookingStatsRow, error)
}

func (r statsRepository) GetBookingStats(ctx context.Context, event uuid.UUID, from, to time.Time) ([]analytics.BookingStatsRow, error) {
	return r.get(ctx, event, from, to)
}

func requestStats(stats service.BookingStatsRepository, events service.EventsRepository, identity *auth.Identity, eventID, rawQuery string) *httptest.ResponseRecorder {
	h := transport.NewHTTPAnalyticsTransport(service.NewAnalyticsService(stats, events))
	router := chi.NewRouter()
	router.Get("/api/v1/organizer/events/{event_id}/analytics/bookings", h.GetBookingStats)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizer/events/"+eventID+"/analytics/bookings?"+rawQuery, nil)
	if identity != nil {
		req = req.WithContext(auth.WithIdentity(req.Context(), *identity))
	}
	rw := httptest.NewRecorder()
	router.ServeHTTP(rw, req)
	return rw
}

func organizer() auth.Identity {
	return auth.Identity{UserID: uuid.New(), OrganizerID: uuid.New(), Roles: []domain.Role{domain.RoleOrganizer}}
}

func TestBookingStatsHTTP(t *testing.T) {
	identity, eventID := organizer(), uuid.New()
	from := time.Date(2026, 11, 8, 12, 0, 0, 123456000, time.FixedZone("MSK", 3*60*60))
	to := from.Add(time.Hour)
	query := url.Values{"from": {from.Format(time.RFC3339Nano)}, "to": {to.Format(time.RFC3339Nano)}}
	var calls []string
	events := eventRepository{get: func(ctx context.Context, event, org, user uuid.UUID) (domain.Event, error) {
		calls = append(calls, "access")
		require.Equal(t, eventID, event)
		require.Equal(t, identity.OrganizerID, org)
		require.Equal(t, identity.UserID, user)
		return domain.Event{ID: event}, nil
	}}
	stats := statsRepository{get: func(ctx context.Context, event uuid.UUID, f, end time.Time) ([]analytics.BookingStatsRow, error) {
		calls = append(calls, "analytics")
		require.Equal(t, eventID, event)
		require.Equal(t, from.UTC(), f)
		require.Equal(t, to.UTC(), end)
		return []analytics.BookingStatsRow{{EventType: booking_events.BookingCreatedType, Bookings: 2, Tickets: 5}}, nil
	}}
	rw := requestStats(stats, events, &identity, eventID.String(), query.Encode())
	require.Equal(t, http.StatusOK, rw.Code, rw.Body.String())
	require.Equal(t, "application/json", rw.Header().Get("Content-Type"))
	require.Equal(t, []string{"access", "analytics"}, calls)
	require.JSONEq(t, fmt.Sprintf(`{
		"event_id": %q, "from": %q, "to": %q,
		"created": {"bookings": 2, "tickets": 5},
		"cancelled": {"bookings": 0, "tickets": 0},
		"expired": {"bookings": 0, "tickets": 0}
	}`, eventID.String(), from.UTC().Format(time.RFC3339Nano), to.UTC().Format(time.RFC3339Nano)), rw.Body.String())
}

func TestBookingStatsHTTPRejectsInvalidRequests(t *testing.T) {
	identity, buyer, noOrg, noUser := organizer(), organizer(), organizer(), organizer()
	buyer.Roles = []domain.Role{domain.RoleBuyer}
	noOrg.OrganizerID = uuid.Nil
	noUser.UserID = uuid.Nil
	eventID := uuid.New().String()
	valid := "from=2026-11-08T00:00:00Z&to=2026-11-09T00:00:00Z"
	for _, tc := range []struct {
		name     string
		identity *auth.Identity
		eventID  string
		query    string
		status   int
	}{
		{"anonymous", nil, eventID, valid, http.StatusUnauthorized},
		{"no user", &noUser, eventID, valid, http.StatusUnauthorized},
		{"buyer", &buyer, eventID, valid, http.StatusForbidden},
		{"no organizer", &noOrg, eventID, valid, http.StatusForbidden},
		{"invalid event", &identity, "bad-id", valid, http.StatusBadRequest},
		{"zero event", &identity, uuid.Nil.String(), valid, http.StatusBadRequest},
		{"missing period", &identity, eventID, "", http.StatusBadRequest},
		{"missing to", &identity, eventID, "from=2026-11-08T00:00:00Z", http.StatusBadRequest},
		{"empty from", &identity, eventID, "from=&to=2026-11-09T00:00:00Z", http.StatusBadRequest},
		{"bad date", &identity, eventID, "from=yesterday&to=2026-11-09T00:00:00Z", http.StatusBadRequest},
		{"no timezone", &identity, eventID, "from=2026-11-08T00:00:00&to=2026-11-09T00:00:00Z", http.StatusBadRequest},
		{"repeated from", &identity, eventID, valid + "&from=2026-11-07T00:00:00Z", http.StatusBadRequest},
		{"repeated to", &identity, eventID, valid + "&to=2026-11-10T00:00:00Z", http.StatusBadRequest},
		{"equal bounds", &identity, eventID, "from=2026-11-08T00:00:00Z&to=2026-11-08T00:00:00Z", http.StatusBadRequest},
		{"reversed bounds", &identity, eventID, "from=2026-11-09T00:00:00Z&to=2026-11-08T00:00:00Z", http.StatusBadRequest},
		{"invalid query encoding", &identity, eventID, valid + "&broken=%zz", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Invalid requests must not reach either storage.
			rw := requestStats(statsRepository{}, eventRepository{}, tc.identity, tc.eventID, tc.query)
			require.Equal(t, tc.status, rw.Code, rw.Body.String())
		})
	}
}

func TestBookingStatsHTTPRepositoryErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		access error
		stats  error
		status int
		code   string
	}{
		{"foreign event", fmt.Errorf("membership: %w", coreerrors.ErrNotFound), nil, http.StatusNotFound, "not_found"},
		{"forbidden", coreerrors.ErrForbidden, nil, http.StatusForbidden, "forbidden"},
		{"postgres down", errors.New("postgres password=hidden"), nil, http.StatusInternalServerError, "internal_error"},
		{"clickhouse down", nil, errors.New("clickhouse password=hidden"), http.StatusInternalServerError, "internal_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			identity := organizer()
			analyticsCalls := 0
			events := eventRepository{get: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Event, error) {
				return domain.Event{}, tc.access
			}}
			stats := statsRepository{get: func(context.Context, uuid.UUID, time.Time, time.Time) ([]analytics.BookingStatsRow, error) {
				analyticsCalls++
				return nil, tc.stats
			}}
			rw := requestStats(stats, events, &identity, uuid.New().String(), "from=2026-11-08T00:00:00Z&to=2026-11-09T00:00:00Z")
			require.Equal(t, tc.status, rw.Code)
			var body struct {
				Code string `json:"code"`
			}
			require.NoError(t, json.Unmarshal(rw.Body.Bytes(), &body))
			require.Equal(t, tc.code, body.Code)
			require.NotContains(t, rw.Body.String(), "password")
			if tc.access != nil {
				require.Zero(t, analyticsCalls)
			}
		})
	}
}
