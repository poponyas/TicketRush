package session_transport_test

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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

func TestCreateSessionHTTPPostgres(t *testing.T) {
	f := testutil.New(t)
	event := f.Event(t, f.OrgID, "Session test")
	foreign := f.Event(t, f.OtherOrgID, "Other event")
	// The fixture owns this venue and cleans it up after its sessions.
	seedSession := f.Session(t, event.ID)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var venueID uuid.UUID
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT venue_id FROM sessions WHERE id=$1`, seedSession).Scan(&venueID))
	f.Exec(t, `DELETE FROM sessions WHERE id=$1`, seedSession)
	h := transport.NewHTTPSessionTransport(service.NewSessionService(repository.NewSessionRepository(&postgres.Pool{Pool: f.Pool, OpTimeout: 5 * time.Second})))
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.With(auth.RequireOrganizer).Post("/api/v1/organizer/events/{event_id}/sessions", h.CreateSession)
	identity := auth.Identity{UserID: f.UserID, OrganizerID: f.OrgID, Roles: []domain.Role{domain.RoleOrganizer}}
	path := func(id string) string { return "/api/v1/organizer/events/" + id + "/sessions" }
	now := time.Now().UTC().Truncate(time.Second)
	body := map[string]any{"venue_id": venueID, "starts_at": now.Add(72 * time.Hour), "sales_start_at": now.Add(-time.Hour), "sales_end_at": now.Add(48 * time.Hour)}
	encode := func(b map[string]any) string {
		data, err := json.Marshal(b)
		require.NoError(t, err)
		return string(data)
	}
	call := func(eventID, body, contentType string, actor *auth.Identity, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path(eventID), strings.NewReader(body))
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		if actor != nil {
			req = req.WithContext(auth.WithIdentity(req.Context(), *actor))
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, status, w.Code, w.Body.String())
		require.Equal(t, "application/json", w.Header().Get("Content-Type"))
		return w
	}
	w := call(event.ID.String(), encode(body), "application/json; charset=utf-8", &identity, 201)
	var saved transport.SessionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &saved))
	require.NotEqual(t, uuid.Nil, saved.ID)
	require.Equal(t, event.ID, saved.EventID)
	require.Equal(t, venueID, saved.VenueID)
	require.Equal(t, domain.SessionDraft, saved.Status)
	require.Nil(t, saved.EndsAt)
	require.False(t, saved.CreatedAt.Before(now))
	var stored domain.Session
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT id,event_id,venue_id,starts_at,ends_at,sales_start_at,sales_end_at,status,created_at FROM sessions WHERE id=$1`, saved.ID).
		Scan(&stored.ID, &stored.EventID, &stored.VenueID, &stored.StartsAt, &stored.EndsAt, &stored.SalesStartAt, &stored.SalesEndAt, &stored.Status, &stored.CreatedAt))
	require.Equal(t, saved.ID, stored.ID)
	require.True(t, saved.StartsAt.Equal(stored.StartsAt))
	require.True(t, saved.CreatedAt.Equal(stored.CreatedAt))
	withEnd := map[string]any{}
	for key, value := range body {
		withEnd[key] = value
	}
	withEnd["ends_at"] = now.Add(74 * time.Hour)
	w = call(event.ID.String(), encode(withEnd), "application/json", &identity, 201)
	var ended transport.SessionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ended))
	require.NotNil(t, ended.EndsAt)
	for _, field := range []string{"venue_id", "starts_at", "sales_start_at", "sales_end_at"} {
		bad := map[string]any{}
		for k, v := range body {
			bad[k] = v
		}
		delete(bad, field)
		call(event.ID.String(), encode(bad), "application/json", &identity, 400)
	}
	for _, change := range []map[string]any{
		{"ends_at": now.Add(72 * time.Hour)},
		{"sales_end_at": now.Add(73 * time.Hour)},
		{"sales_start_at": now.Add(48 * time.Hour)},
		{"sales_end_at": now.Add(-time.Minute)},
		{"starts_at": "not-a-date"},
		{"venue_id": "not-a-uuid"},
		{"created_at": now},
		{"status": "scheduled"},
	} {
		bad := map[string]any{}
		for k, v := range body {
			bad[k] = v
		}
		for k, v := range change {
			bad[k] = v
		}
		call(event.ID.String(), encode(bad), "application/json", &identity, 400)
	}
	for _, raw := range []string{"", `{}`, `null`, encode(body) + " {}"} {
		call(event.ID.String(), raw, "application/json", &identity, 400)
	}
	call(event.ID.String(), `{"starts_at":"`+strings.Repeat("x", 1<<20)+`"}`, "application/json", &identity, 413)
	for _, ct := range []string{"", "text/plain"} {
		call(event.ID.String(), encode(body), ct, &identity, 415)
	}
	call("invalid", encode(body), "application/json", &identity, 400)
	call(uuid.New().String(), encode(body), "application/json", &identity, 404)
	call(foreign.ID.String(), encode(body), "application/json", &identity, 404)
	call(event.ID.String(), encode(body), "application/json", nil, 401)
	outsider := auth.Identity{UserID: f.OtherUserID, OrganizerID: f.OrgID, Roles: []domain.Role{domain.RoleOrganizer}}
	call(event.ID.String(), encode(body), "application/json", &outsider, 404)
	f.Exec(t, `INSERT INTO organizer_memberships(organizer_id,user_id,role) VALUES($1,$2,'controller')`, f.OrgID, f.OtherUserID)
	call(event.ID.String(), encode(body), "application/json", &outsider, 404)
	buyer := identity
	buyer.Roles = []domain.Role{domain.RoleBuyer}
	call(event.ID.String(), encode(body), "application/json", &buyer, 403)
	missingVenue := map[string]any{}
	for k, v := range body {
		missingVenue[k] = v
	}
	missingVenue["venue_id"] = uuid.New()
	call(event.ID.String(), encode(missingVenue), "application/json", &identity, 404)
	for _, status := range []domain.EventStatus{domain.EventPublished, domain.EventArchived} {
		f.Exec(t, `UPDATE events SET status=$1 WHERE id=$2`, status, event.ID)
		call(event.ID.String(), encode(body), "application/json", &identity, 409)
	}
	var count int
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE event_id IN ($1,$2)`, event.ID, foreign.ID).Scan(&count))
	require.Equal(t, 2, count, "failed requests must not create sessions")
}
