package session_transport_test

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	"github.com/poponyas/TicketRush/internal/feature/events/testutil"
	repository "github.com/poponyas/TicketRush/internal/feature/session/repository"
	service "github.com/poponyas/TicketRush/internal/feature/session/service"
	transport "github.com/poponyas/TicketRush/internal/feature/session/transport"
	"github.com/stretchr/testify/require"
)

func TestSessionHTTPReadsPostgres(t *testing.T) {
	f := testutil.New(t)
	event := f.Event(t, f.OrgID, "Own event")
	empty := f.Event(t, f.OrgID, "Empty event")
	other := f.Event(t, f.OrgID, "Another own event")
	foreign := f.Event(t, f.OtherOrgID, "Foreign event")
	otherSession := f.Session(t, other.ID)
	foreignSession := f.Session(t, foreign.ID)
	ids := make([]uuid.UUID, 4)
	base := time.Date(2027, 1, 10, 18, 0, 0, 0, time.UTC)
	for i, status := range []domain.SessionStatus{domain.SessionDraft, domain.SessionScheduled, domain.SessionCancelled, domain.SessionCompleted} {
		ids[i] = f.Session(t, event.ID)
		hours := i
		if i == 2 {
			hours = 1
		}
		f.Exec(t, `UPDATE sessions SET starts_at=$1,status=$2 WHERE id=$3`, base.Add(time.Duration(hours)*time.Hour), status, ids[i])
	}
	expected := append([]uuid.UUID(nil), ids...)
	if expected[1].String() > expected[2].String() {
		expected[1], expected[2] = expected[2], expected[1]
	}
	h := transport.NewHTTPSessionTransport(service.NewSessionService(repository.NewSessionRepository(&postgres.Pool{Pool: f.Pool, OpTimeout: 5 * time.Second})))
	r := chi.NewRouter()
	r.With(auth.RequireOrganizer).Get("/api/v1/organizer/events/{event_id}/sessions", h.GetSessions)
	r.With(auth.RequireOrganizer).Get("/api/v1/organizer/events/{event_id}/sessions/{session_id}", h.GetSession)
	path := func(eventID string) string { return "/api/v1/organizer/events/" + eventID + "/sessions" }
	identity := auth.Identity{UserID: f.UserID, OrganizerID: f.OrgID, Roles: []domain.Role{domain.RoleOrganizer}}
	call := func(url string, actor *auth.Identity, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, url, nil)
		if actor != nil {
			req = req.WithContext(auth.WithIdentity(req.Context(), *actor))
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, status, w.Code, w.Body.String())
		require.Equal(t, "application/json", w.Header().Get("Content-Type"))
		return w
	}
	w := call(path(event.ID.String())+"/"+ids[0].String(), &identity, 200)
	var single transport.SessionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &single))
	require.Equal(t, ids[0], single.ID)
	require.Equal(t, event.ID, single.EventID)
	require.Nil(t, single.EndsAt)
	require.Equal(t, domain.SessionDraft, single.Status)
	// Reading non-draft sessions and events is allowed in the organizer cabinet.
	f.Exec(t, `UPDATE events SET status='published' WHERE id=$1`, event.ID)
	for _, id := range ids {
		call(path(event.ID.String())+"/"+id.String(), &identity, 200)
	}
	for _, tc := range []struct {
		query         string
		want          []uuid.UUID
		limit, offset int
	}{
		{"", expected, 20, 0}, {"?limit=2&offset=0", expected[:2], 2, 0}, {"?limit=2&offset=2", expected[2:], 2, 2},
	} {
		w = call(path(event.ID.String())+tc.query, &identity, 200)
		var page transport.SessionsListResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
		require.Equal(t, tc.limit, page.Limit)
		require.Equal(t, tc.offset, page.Offset)
		require.Len(t, page.Items, len(tc.want))
		for i, item := range page.Items {
			require.Equal(t, tc.want[i], item.ID)
			require.Equal(t, event.ID, item.EventID)
		}
	}
	w = call(path(empty.ID.String()), &identity, 200)
	require.JSONEq(t, `{"items":[],"limit":20,"offset":0}`, w.Body.String())
	w = call(path(event.ID.String())+"?offset=100", &identity, 200)
	require.JSONEq(t, `{"items":[],"limit":20,"offset":100}`, w.Body.String())
	for _, eventID := range []string{uuid.New().String(), foreign.ID.String()} {
		call(path(eventID), &identity, 404)
		call(path(eventID)+"/"+ids[0].String(), &identity, 404)
	}
	for _, id := range []uuid.UUID{otherSession, foreignSession, uuid.New()} {
		call(path(event.ID.String())+"/"+id.String(), &identity, 404)
	}
	for _, bad := range []string{"invalid", uuid.Nil.String()} {
		call(path(bad), &identity, 400)
		call(path(bad)+"/"+ids[0].String(), &identity, 400)
		call(path(event.ID.String())+"/"+bad, &identity, 400)
	}
	for _, query := range []string{"limit=0", "limit=-1", "limit=101", "offset=-1", "limit=abc", "offset=abc", "limit=", "limit=1&limit=2"} {
		call(path(event.ID.String())+"?"+query, &identity, 400)
	}
	outsider := auth.Identity{UserID: f.OtherUserID, OrganizerID: f.OrgID, Roles: []domain.Role{domain.RoleOrganizer}}
	for _, url := range []string{path(event.ID.String()), path(event.ID.String()) + "/" + ids[0].String()} {
		call(url, nil, 401)
		buyer := identity
		buyer.Roles = []domain.Role{domain.RoleBuyer}
		call(url, &buyer, 403)
		call(url, &outsider, 404)
	}
	f.Exec(t, `INSERT INTO organizer_memberships(organizer_id,user_id,role) VALUES($1,$2,'controller')`, f.OrgID, f.OtherUserID)
	call(path(event.ID.String()), &outsider, 404)
	call(path(event.ID.String())+"/"+ids[0].String(), &outsider, 404)
	// A user can switch to a different organizer they actually belong to.
	otherIdentity := identity
	otherIdentity.OrganizerID = f.OtherOrgID
	call(path(foreign.ID.String())+"/"+foreignSession.String(), &otherIdentity, 200)
	// Revocation must apply even if the request still carries the organizer role.
	f.Exec(t, `DELETE FROM organizer_memberships WHERE organizer_id=$1 AND user_id=$2`, f.OrgID, f.UserID)
	call(path(event.ID.String()), &identity, 404)
	call(path(event.ID.String())+"/"+ids[0].String(), &identity, 404)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := repository.NewSessionRepository(&postgres.Pool{Pool: f.Pool, OpTimeout: time.Second})
	_, err := repo.GetSession(ctx, event.ID, ids[0], f.OrgID, f.UserID)
	require.Error(t, err)
	_, err = repo.GetSessions(ctx, event.ID, f.OrgID, f.UserID, 20, 0)
	require.Error(t, err)
}
