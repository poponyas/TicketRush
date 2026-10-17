package session_transport_test

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	"github.com/poponyas/TicketRush/internal/feature/events/testutil"
	repository "github.com/poponyas/TicketRush/internal/feature/session/repository"
	service "github.com/poponyas/TicketRush/internal/feature/session/service"
	transport "github.com/poponyas/TicketRush/internal/feature/session/transport"
	"github.com/stretchr/testify/require"
)

func TestUpdateSessionHTTPPostgres(t *testing.T) {
	f := testutil.New(t)
	event := f.Event(t, f.OrgID, "Update session")
	id := f.Session(t, event.ID)
	repo := repository.NewSessionRepository(&postgres.Pool{Pool: f.Pool, OpTimeout: 5 * time.Second})
	h := transport.NewHTTPSessionTransport(service.NewSessionService(repo))
	router := chi.NewRouter()
	router.With(auth.RequireOrganizer).Patch("/api/v1/organizer/events/{event_id}/sessions/{session_id}", h.UpdateSession)
	identity := auth.Identity{UserID: f.UserID, OrganizerID: f.OrgID, Roles: []domain.Role{domain.RoleOrganizer}}
	path := func(eventID, sessionID uuid.UUID) string {
		return "/api/v1/organizer/events/" + eventID.String() + "/sessions/" + sessionID.String()
	}
	call := func(url, body, ct string, actor *auth.Identity, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPatch, url, strings.NewReader(body))
		req.Header.Set("Content-Type", ct)
		if actor != nil {
			req = req.WithContext(auth.WithIdentity(req.Context(), *actor))
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, status, w.Code, w.Body.String())
		return w
	}
	url := path(event.ID, id)
	start := time.Now().UTC().AddDate(1, 0, 0).Truncate(time.Second)
	end := start.Add(3 * time.Hour)
	body := fmt.Sprintf(`{"starts_at":%q,"ends_at":%q}`, start.Format(time.RFC3339), end.Format(time.RFC3339))
	w := call(url, body, "application/json", &identity, 200)
	var result transport.SessionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.True(t, start.Equal(result.StartsAt))
	require.True(t, end.Equal(*result.EndsAt))
	created := result.CreatedAt
	w = call(url, `{"ends_at":null}`, "application/json; charset=utf-8", &identity, 200)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.Nil(t, result.EndsAt)
	require.True(t, start.Equal(result.StartsAt))
	require.Equal(t, created, result.CreatedAt)
	call(url, fmt.Sprintf(`{"ends_at":%q}`, end.Format(time.RFC3339)), "application/json", &identity, 200)
	// A start-only patch preserves the existing end.
	w = call(url, fmt.Sprintf(`{"starts_at":%q}`, start.Add(time.Hour).Format(time.RFC3339)), "application/json", &identity, 200)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.True(t, end.Equal(*result.EndsAt))
	before, err := repo.GetSession(context.Background(), event.ID, id, f.OrgID, f.UserID)
	require.NoError(t, err)
	for _, bad := range []string{`{}`, `null`, `[]`, `{"starts_at":null}`, `{"starts_at":"0001-01-01T00:00:00Z"}`, `{"ends_at":"0001-01-01T00:00:00Z"}`, `{"starts_at":"2020-01-01T00:00:00Z"}`, `{"ends_at":"2020-01-01T00:00:00Z"}`, `{"starts_at":"bad"}`, `{"venue_id":"ignored"}`, `{"ends_at":null} {}`, `{"ends_at":null,"ends_at":null}`, `{"Starts_at":"2028-01-01T00:00:00Z"}`, `{"ends_at":123}`, `{"ends_at":"2028-01-01T00:00:00"}`} {
		call(url, bad, "application/json", &identity, 400)
	}
	call(url, body, "text/plain", &identity, 415)
	call(url, `{"ends_at":"`+strings.Repeat("x", 1<<20)+`"}`, "application/json", &identity, 413)
	after, err := repo.GetSession(context.Background(), event.ID, id, f.OrgID, f.UserID)
	require.NoError(t, err)
	require.Equal(t, before, after)
	call(url, body, "application/json", nil, 401)
	buyer := identity
	buyer.Roles = []domain.Role{domain.RoleBuyer}
	call(url, body, "application/json", &buyer, 403)
	outsider := identity
	outsider.UserID = f.OtherUserID
	call(url, body, "application/json", &outsider, 404)
	f.Exec(t, `INSERT INTO organizer_memberships(organizer_id,user_id,role) VALUES($1,$2,'controller')`, f.OrgID, f.OtherUserID)
	call(url, body, "application/json", &outsider, 404)
	foreign := f.Event(t, f.OtherOrgID, "Foreign")
	foreignSession := f.Session(t, foreign.ID)
	call(path(event.ID, foreignSession), body, "application/json", &identity, 404)
	call(path(foreign.ID, foreignSession), body, "application/json", &identity, 404)
	call(path(event.ID, uuid.New()), body, "application/json", &identity, 404)
	call(path(event.ID, uuid.Nil), body, "application/json", &identity, 400)
	call(strings.Replace(url, id.String(), "bad", 1), body, "application/json", &identity, 400)
	for _, status := range []string{"scheduled", "cancelled", "completed"} {
		f.Exec(t, `UPDATE sessions SET status=$1 WHERE id=$2`, status, id)
		call(url, body, "application/json", &identity, 409)
	}
	f.Exec(t, `UPDATE sessions SET status='draft' WHERE id=$1`, id)
	f.Exec(t, `UPDATE events SET status='published' WHERE id=$1`, event.ID)
	call(url, body, "application/json", &identity, 409)
	f.Exec(t, `UPDATE events SET status='draft' WHERE id=$1`, event.ID)
	f.Exec(t, `DELETE FROM organizer_memberships WHERE organizer_id=$1 AND user_id=$2`, f.OrgID, f.UserID)
	call(url, body, "application/json", &identity, 404)
}

func TestUpdateSessionConcurrentPostgres(t *testing.T) {
	f := testutil.New(t)
	event := f.Event(t, f.OrgID, "Concurrent update")
	id := f.Session(t, event.ID)
	repo := repository.NewSessionRepository(&postgres.Pool{Pool: f.Pool, OpTimeout: 5 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	expected, err := repo.GetSession(ctx, event.ID, id, f.OrgID, f.UserID)
	require.NoError(t, err)
	// Both requests share the same snapshot, then write different fields.
	end := expected.StartsAt.Add(3 * time.Hour)
	first, second := expected, expected
	first.StartsAt = expected.StartsAt.Add(time.Hour)
	second.EndsAt = &end
	candidates := []domain.Session{first, second}
	results := make([]error, 2)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range candidates {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, results[i] = repo.UpdateSession(ctx, candidates[i], expected, f.OrgID, f.UserID)
		}(i)
	}
	close(start)
	wg.Wait()
	winner := 0
	if results[0] != nil {
		winner = 1
	}
	require.NoError(t, results[winner])
	require.ErrorIs(t, results[1-winner], coreerrors.ErrConflict)
	saved, err := repo.GetSession(ctx, event.ID, id, f.OrgID, f.UserID)
	require.NoError(t, err)
	require.True(t, candidates[winner].StartsAt.Equal(saved.StartsAt))
	require.Equal(t, candidates[winner].EndsAt, saved.EndsAt)
	// Deletion between read and write remains 404 rather than a conflict.
	f.Exec(t, `DELETE FROM sessions WHERE id=$1`, id)
	_, err = repo.UpdateSession(ctx, first, expected, f.OrgID, f.UserID)
	require.ErrorIs(t, err, coreerrors.ErrNotFound)
}
