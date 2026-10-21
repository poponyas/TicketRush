package events_transport_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/events/service"
	transport "github.com/poponyas/TicketRush/internal/feature/events/transport"
	"github.com/stretchr/testify/require"
)

type publicEventService struct {
	stubService
	get func(context.Context, uuid.UUID) (domain.Event, error)
}

func (s *publicEventService) GetPublicEvent(ctx context.Context, id uuid.UUID) (domain.Event, error) {
	return s.get(ctx, id)
}

func TestGetPublicEventHTTP(t *testing.T) {
	event := domain.NewEvent(uuid.New(), "Go Live", "Description", "conference")
	event.Status = domain.EventPublished
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"anonymous success", nil, http.StatusOK},
		{"not found", fmt.Errorf("repository: %w", coreerrors.ErrNotFound), http.StatusNotFound},
		{"internal error", errors.New("postgres password=secret"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			svc := &publicEventService{get: func(ctx context.Context, id uuid.UUID) (domain.Event, error) {
				calls++
				require.Equal(t, event.ID, id)
				return event, tc.err
			}}
			router := chi.NewRouter()
			router.Get("/api/v1/events/{event_id}", transport.NewEventsHTTPTransport(svc).GetPublicEvent)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/events/"+event.ID.String(), nil))
			require.Equal(t, tc.status, w.Code)
			require.Equal(t, 1, calls)
			require.Equal(t, "application/json", w.Header().Get("Content-Type"))
			require.NotContains(t, w.Body.String(), "secret")
			if tc.err == nil {
				var body transport.EventResponse
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				require.Equal(t, event.ID, body.ID)
				require.Equal(t, event.Title, body.Title)
				require.Equal(t, domain.EventPublished, body.Status)
			}
		})
	}
}

func TestGetPublicEventHTTPInvalidID(t *testing.T) {
	svc := &publicEventService{get: func(context.Context, uuid.UUID) (domain.Event, error) {
		t.Fatal("malformed UUID must not reach the service")
		return domain.Event{}, nil
	}}
	router := chi.NewRouter()
	router.Get("/api/v1/events/{event_id}", transport.NewEventsHTTPTransport(svc).GetPublicEvent)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/events/not-a-uuid", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)

	// A syntactically valid zero UUID is rejected by the real service.
	router = chi.NewRouter()
	router.Get("/api/v1/events/{event_id}", transport.NewEventsHTTPTransport(service.NewEventsService(nil)).GetPublicEvent)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/events/"+uuid.Nil.String(), nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
}
