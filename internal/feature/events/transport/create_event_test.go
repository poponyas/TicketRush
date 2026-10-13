package events_transport_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	service "github.com/poponyas/TicketRush/internal/feature/events/service"
	transport "github.com/poponyas/TicketRush/internal/feature/events/transport"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type stubService struct {
	input service.CreateEventInput
	calls int
	err   error
}

func (s *stubService) GetEvent(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Event, error) {
	panic("unexpected GetEvent call in CreateEvent test")
}

func (s *stubService) GetEvents(context.Context, *int, *int, uuid.UUID, uuid.UUID) (service.EventsPage, error) {
	panic("unexpected GetEvents call in CreateEvent test")
}

func (s *stubService) UpdateEvent(context.Context, service.UpdateEventInput, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Event, error) {
	panic("unexpected UpdateEvent call in CreateEvent test")
}

func (s *stubService) DeleteEvent(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	panic("unexpected DeleteEvent call in CreateEvent test")
}

func (s *stubService) CreateEvent(ctx context.Context, input service.CreateEventInput) (domain.Event, error) {
	s.calls++
	s.input = input
	e := domain.NewEvent(input.OrganizerID, input.Title, input.Description, input.Category)
	e.CreatedAt = time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
	e.UpdatedAt = e.CreatedAt
	return e, s.err
}
func organizerRequest(body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/organizer/events", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r.WithContext(auth.WithIdentity(r.Context(), auth.Identity{
		UserID: uuid.MustParse("22222222-2222-4222-8222-222222222222"), OrganizerID: uuid.MustParse("33333333-3333-4333-8333-333333333333"), Roles: []domain.Role{domain.RoleOrganizer},
	}))
}
func TestCreateEventHTTP(t *testing.T) {
	s := &stubService{}
	h := transport.NewEventsHTTPTransport(s)
	w := httptest.NewRecorder()
	h.CreateEvent(w, organizerRequest(`{"name":"Go Live","description":"Go","category":"conference"}`))
	require.Equal(t, http.StatusCreated, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var response transport.EventResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, domain.EventDraft, response.Status)
	require.False(t, response.CreatedAt.IsZero())
	require.Equal(t, "/api/v1/organizer/events/"+response.ID.String(), w.Header().Get("Location"))
	require.Equal(t, uuid.MustParse("33333333-3333-4333-8333-333333333333"), s.input.OrganizerID)
	require.NotContains(t, w.Body.String(), "total_tickets")
}
func TestCreateEventRejectsInvalidBodies(t *testing.T) {
	cases := []struct {
		name, body string
		status     int
	}{
		{"malformed", `{"name":`, 400}, {"trailing", `{"name":"Go"} {}`, 400},
		{"unknown", `{"name":"Go","organizer_id":"other"}`, 400},
		{"old_inventory", `{"name":"Go","total_tickets":20}`, 400},
		{"oversized", `{"name":"` + strings.Repeat("x", (1<<20)+1) + `"}`, 413},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &stubService{}
			h := transport.NewEventsHTTPTransport(s)
			w := httptest.NewRecorder()
			h.CreateEvent(w, organizerRequest(tc.body))
			require.Equal(t, tc.status, w.Code)
			require.Zero(t, s.calls)
		})
	}
}
func TestCreateEventRequiresIdentityAndRole(t *testing.T) {
	s := &stubService{}
	h := transport.NewEventsHTTPTransport(s)
	r := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	// A client-controlled role cookie does not authenticate the request.
	r.AddCookie(&http.Cookie{Name: "role", Value: "organizer"})
	h.CreateEvent(w, r)
	require.Equal(t, 401, w.Code)
	r = r.WithContext(auth.WithIdentity(r.Context(), auth.Identity{UserID: uuid.New(), Roles: []domain.Role{domain.RoleBuyer}}))
	w = httptest.NewRecorder()
	h.CreateEvent(w, r)
	require.Equal(t, 403, w.Code)
	require.Zero(t, s.calls)
}
func TestCreateEventDoesNotLeakInternalErrors(t *testing.T) {
	s := &stubService{err: fmt.Errorf("postgres password=secret details")}
	h := transport.NewEventsHTTPTransport(s)
	w := httptest.NewRecorder()
	h.CreateEvent(w, organizerRequest(`{"name":"Go"}`))
	require.Equal(t, 500, w.Code)
	require.NotContains(t, w.Body.String(), "secret")
	require.NotContains(t, w.Body.String(), "postgres")
}
