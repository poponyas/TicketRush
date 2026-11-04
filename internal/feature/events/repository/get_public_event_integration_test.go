package events_repository_test

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/events/service"
	"github.com/poponyas/TicketRush/internal/feature/events/testutil"
	transport "github.com/poponyas/TicketRush/internal/feature/events/transport"
	"github.com/stretchr/testify/require"
)

func TestGetPublicEventPostgres(t *testing.T) {
	f := testutil.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	published := f.Event(t, f.OrgID, "Published event")
	f.Exec(t, `UPDATE events SET status='published', poster_key='posters/event.png' WHERE id=$1`, published.ID)
	draft := f.Event(t, f.OrgID, "Draft event")
	archived := f.Event(t, f.OtherOrgID, "Archived event")
	f.Exec(t, `UPDATE events SET status='archived' WHERE id=$1`, archived.ID)

	// Exercise HTTP -> service -> repository without any identity or token.
	router := chi.NewRouter()
	router.Get("/api/v1/events/{event_id}", transport.NewEventsHTTPTransport(service.NewEventsService(f.Repo, nil)).GetPublicEvent)
	for _, tc := range []struct {
		name   string
		id     uuid.UUID
		status int
	}{
		{"published", published.ID, http.StatusOK},
		{"draft hidden", draft.ID, http.StatusNotFound},
		{"archived hidden", archived.ID, http.StatusNotFound},
		{"missing", uuid.New(), http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event, err := f.Repo.GetPublicEvent(ctx, tc.id)
			if tc.status == http.StatusOK {
				require.NoError(t, err)
				require.Equal(t, published.ID, event.ID)
				require.Equal(t, published.OrganizerID, event.OrganizerID)
				require.Equal(t, published.Title, event.Title)
				require.Equal(t, published.Description, event.Description)
				require.Equal(t, published.Category, event.Category)
				require.Equal(t, "posters/event.png", event.PosterKey)
				require.Equal(t, domain.EventPublished, event.Status)
				require.True(t, published.CreatedAt.Equal(event.CreatedAt))
				require.True(t, published.UpdatedAt.Equal(event.UpdatedAt))
			} else {
				require.ErrorIs(t, err, coreerrors.ErrNotFound)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/events/"+tc.id.String(), nil))
			require.Equal(t, tc.status, w.Code)
			if tc.status == http.StatusOK {
				var body transport.EventResponse
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				require.Equal(t, published.ID, body.ID)
				require.Equal(t, domain.EventPublished, body.Status)
			} else {
				require.NotContains(t, w.Body.String(), "Draft event")
				require.NotContains(t, w.Body.String(), "Archived event")
			}
		})
	}
}
