package events_transport_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	service "github.com/poponyas/TicketRush/internal/feature/events/service"
	transport "github.com/poponyas/TicketRush/internal/feature/events/transport"
	"github.com/stretchr/testify/require"
)

type publicListService struct {
	stubService
	list func(context.Context, *int, *int) (service.EventsPage, error)
}

func (s *publicListService) GetPublicEvents(ctx context.Context, limit, offset *int) (service.EventsPage, error) {
	return s.list(ctx, limit, offset)
}

func TestGetPublicEventsHTTPWithoutAuthentication(t *testing.T) {
	for _, tc := range []struct {
		name, query   string
		limit, offset int
		empty         bool
	}{
		{"defaults", "", 20, 0, false},
		{"explicit pagination", "?limit=3&offset=7", 3, 7, false},
		{"empty list", "", 20, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event := domain.NewEvent(uuid.New(), "Go Live", "Description", "conference")
			event.Status = domain.EventPublished
			calls := 0
			svc := &publicListService{list: func(ctx context.Context, limit, offset *int) (service.EventsPage, error) {
				calls++
				if tc.query == "" {
					require.Nil(t, limit)
					require.Nil(t, offset)
				} else {
					require.NotNil(t, limit)
					require.NotNil(t, offset)
					require.Equal(t, tc.limit, *limit)
					require.Equal(t, tc.offset, *offset)
				}
				page := service.EventsPage{Limit: tc.limit, Offset: tc.offset}
				if !tc.empty {
					page.Events = []domain.Event{event}
				}
				return page, nil
			}}
			router := chi.NewRouter()
			router.Get("/api/v1/events", transport.NewEventsHTTPTransport(svc).GetPublicEvents)
			w := httptest.NewRecorder()
			// No identity, bearer token or role cookie: this is a public endpoint.
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/events"+tc.query, nil))
			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, "application/json", w.Header().Get("Content-Type"))
			require.Equal(t, 1, calls)
			var body transport.EventsListResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Equal(t, tc.limit, body.Limit)
			require.Equal(t, tc.offset, body.Offset)
			if tc.empty {
				require.NotNil(t, body.Items)
				require.Empty(t, body.Items)
				require.Contains(t, w.Body.String(), `"items":[]`)
			} else {
				require.Len(t, body.Items, 1)
				require.Equal(t, event.ID, body.Items[0].ID)
				require.Equal(t, event.Title, body.Items[0].Title)
				require.Equal(t, domain.EventPublished, body.Items[0].Status)
			}
		})
	}
}

func TestGetPublicEventsHTTPRejectsMalformedPagination(t *testing.T) {
	for _, query := range []string{
		"?limit=abc", "?offset=abc", "?limit=", "?offset=",
		"?limit=1&limit=2", "?offset=1&offset=2",
		"?limit=99999999999999999999999999999999999",
	} {
		t.Run(query, func(t *testing.T) {
			svc := &publicListService{list: func(context.Context, *int, *int) (service.EventsPage, error) {
				t.Fatal("malformed pagination must not reach the service")
				return service.EventsPage{}, nil
			}}
			w := httptest.NewRecorder()
			transport.NewEventsHTTPTransport(svc).GetPublicEvents(w, httptest.NewRequest(http.MethodGet, "/api/v1/events"+query, nil))
			require.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestGetPublicEventsHTTPServiceValidation(t *testing.T) {
	// Exercise the real service's range checks through HTTP. The repository must
	// never be called, so a nil repository is sufficient for these invalid inputs.
	handler := transport.NewEventsHTTPTransport(service.NewEventsService(nil, nil))
	for _, query := range []string{"?limit=0", "?limit=-1", "?limit=101", "?offset=-1"} {
		t.Run(query, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.GetPublicEvents(w, httptest.NewRequest(http.MethodGet, "/api/v1/events"+query, nil))
			require.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestGetPublicEventsHTTPDoesNotLeakDatabaseErrors(t *testing.T) {
	svc := &publicListService{list: func(context.Context, *int, *int) (service.EventsPage, error) {
		return service.EventsPage{}, errors.New("postgres password=secret")
	}}
	w := httptest.NewRecorder()
	transport.NewEventsHTTPTransport(svc).GetPublicEvents(w, httptest.NewRequest(http.MethodGet, "/api/v1/events", nil))
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.NotContains(t, w.Body.String(), "postgres")
	require.NotContains(t, w.Body.String(), "secret")
}
