package events_transport_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	service "github.com/poponyas/TicketRush/internal/feature/events/service"
	"github.com/poponyas/TicketRush/internal/feature/events/testutil"
	transport "github.com/poponyas/TicketRush/internal/feature/events/transport"
	"github.com/stretchr/testify/require"
)

const eventsURL = "/api/v1/organizer/events"

// No service or repository mocks: requests exercise chi -> transport -> service -> PostgreSQL.
func integrationRouter(f *testutil.Fixture) http.Handler {
	h := transport.NewEventsHTTPTransport(service.NewEventsService(f.Repo))
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(middleware.RequestIDHeader, middleware.GetReqID(r.Context()))
			next.ServeHTTP(w, r)
		})
	})
	r.With(auth.RequireOrganizer).Post(eventsURL, h.CreateEvent)
	r.With(auth.RequireOrganizer).Get(eventsURL, h.GetEvents)
	r.With(auth.RequireOrganizer).Get(eventsURL+"/{event_id}", h.GetEvent)
	r.With(auth.RequireOrganizer).Patch(eventsURL+"/{event_id}", h.UpdateEvent)
	r.With(auth.RequireOrganizer).Delete(eventsURL+"/{event_id}", h.DeleteEvent)
	return r
}

func httpCall(t *testing.T, h http.Handler, identity *auth.Identity, method, path, body, contentType string, status int) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if identity != nil {
		req = req.WithContext(auth.WithIdentity(req.Context(), *identity))
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	require.Equal(t, status, w.Code, w.Body.String())
	if status == http.StatusNoContent {
		require.Empty(t, w.Body.String())
	} else {
		require.Equal(t, "application/json", w.Header().Get("Content-Type"))
		require.True(t, jsontext.Value(w.Body.Bytes()).IsValid(), w.Body.String())
	}
	if status >= 400 {
		var response struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.NotEmpty(t, response.Code)
		require.NotEmpty(t, response.Message)
		require.NotEmpty(t, response.RequestID)
		require.Equal(t, w.Header().Get(middleware.RequestIDHeader), response.RequestID)
		require.NotContains(t, w.Body.String(), "SQLSTATE")
		require.NotContains(t, w.Body.String(), "password=")
	}
	return w
}

func organizer(f *testutil.Fixture) *auth.Identity {
	return &auth.Identity{UserID: f.UserID, OrganizerID: f.OrgID, Roles: []domain.Role{domain.RoleOrganizer}}
}

func TestEventsHTTPPostgresLifecycle(t *testing.T) {
	f := testutil.New(t)
	h := integrationRouter(f)
	identity := organizer(f)
	w := httpCall(t, h, identity, http.MethodGet, eventsURL, "", "", 200)
	require.JSONEq(t, `{"items":[],"limit":20,"offset":0}`, w.Body.String())
	w = httpCall(t, h, identity, http.MethodPost, eventsURL, `{"name":" Go Live ","description":"Keep me","category":"conference"}`, "application/json; charset=utf-8", 201)
	var created transport.EventResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.NotEqual(t, uuid.Nil, created.ID)
	require.Equal(t, f.OrgID, created.OrganizerID)
	require.Equal(t, "Go Live", created.Title)
	require.Equal(t, domain.EventDraft, created.Status)
	require.False(t, created.CreatedAt.IsZero())
	path := eventsURL + "/" + created.ID.String()
	require.Equal(t, path, w.Header().Get("Location"))
	w = httpCall(t, h, identity, http.MethodGet, path, "", "", 200)
	var fetched transport.EventResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &fetched))
	require.Equal(t, created, fetched)
	w = httpCall(t, h, identity, http.MethodPatch, path, `{"name":" Updated ","category":""}`, "application/json", 200)
	var updated transport.EventResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	require.Equal(t, "Updated", updated.Title)
	require.Equal(t, "Keep me", updated.Description)
	require.Empty(t, updated.Category)
	require.Equal(t, created.CreatedAt, updated.CreatedAt)
	require.True(t, updated.UpdatedAt.After(created.UpdatedAt))
	second := f.Event(t, f.OrgID, "Second")
	foreign := f.Event(t, f.OtherOrgID, "Foreign")
	f.Exec(t, `UPDATE events SET created_at='2026-01-01 00:00+00' WHERE id=$1`, created.ID)
	f.Exec(t, `UPDATE events SET created_at='2026-01-02 00:00+00' WHERE id=$1`, second.ID)
	f.Exec(t, `UPDATE events SET created_at='2026-01-03 00:00+00' WHERE id=$1`, foreign.ID)
	for _, tc := range []struct {
		query string
		want  uuid.UUID
	}{{"?limit=1&offset=0", second.ID}, {"?limit=1&offset=1", created.ID}} {
		w = httpCall(t, h, identity, http.MethodGet, eventsURL+tc.query, "", "", 200)
		var page transport.EventsListResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
		require.Equal(t, 1, page.Limit)
		require.Len(t, page.Items, 1)
		require.Equal(t, tc.want, page.Items[0].ID)
		require.Equal(t, f.OrgID, page.Items[0].OrganizerID)
	}
	w = httpCall(t, h, identity, http.MethodGet, eventsURL+"?offset=100", "", "", 200)
	require.JSONEq(t, `{"items":[],"limit":20,"offset":100}`, w.Body.String())
	httpCall(t, h, identity, http.MethodDelete, path, "", "", 204)
	httpCall(t, h, identity, http.MethodGet, path, "", "", 404)
	httpCall(t, h, identity, http.MethodDelete, path, "", "", 404)
}

func TestEventsHTTPPostgresAccess(t *testing.T) {
	f := testutil.New(t)
	h := integrationRouter(f)
	e := f.Event(t, f.OrgID, "Own")
	foreign := f.Event(t, f.OtherOrgID, "Foreign")
	path := eventsURL + "/" + e.ID.String()
	for _, method := range []string{http.MethodGet, http.MethodPatch, http.MethodDelete} {
		httpCall(t, h, nil, method, path, `{"name":"Change"}`, "application/json", 401)
		buyer := &auth.Identity{UserID: f.UserID, OrganizerID: f.OrgID, Roles: []domain.Role{domain.RoleBuyer}}
		httpCall(t, h, buyer, method, path, `{"name":"Change"}`, "application/json", 403)
		httpCall(t, h, organizer(f), method, eventsURL+"/"+foreign.ID.String(), `{"name":"Change"}`, "application/json", 404)
		outsider := &auth.Identity{UserID: f.OtherUserID, OrganizerID: f.OrgID, Roles: []domain.Role{domain.RoleOrganizer}}
		httpCall(t, h, outsider, method, path, `{"name":"Change"}`, "application/json", 404)
	}
	outsider := &auth.Identity{UserID: f.OtherUserID, OrganizerID: f.OrgID, Roles: []domain.Role{domain.RoleOrganizer}}
	httpCall(t, h, outsider, http.MethodGet, eventsURL, "", "", 403)
	httpCall(t, h, outsider, http.MethodPost, eventsURL, `{"name":"Denied"}`, "application/json", 403)
	f.Exec(t, `INSERT INTO organizer_memberships(organizer_id,user_id,role) VALUES($1,$2,'controller')`, f.OrgID, f.OtherUserID)
	httpCall(t, h, outsider, http.MethodPatch, path, `{"name":"Denied"}`, "application/json", 404)
	httpCall(t, h, outsider, http.MethodDelete, path, "", "", 404)
	for _, status := range []domain.EventStatus{domain.EventPublished, domain.EventArchived} {
		f.Exec(t, `UPDATE events SET status=$1 WHERE id=$2`, status, e.ID)
		httpCall(t, h, organizer(f), http.MethodPatch, path, `{"name":"Denied"}`, "application/json", 409)
		httpCall(t, h, organizer(f), http.MethodDelete, path, "", "", 409)
		// The organizer can read their own non-draft cards.
		httpCall(t, h, organizer(f), http.MethodGet, path, "", "", 200)
		w := httpCall(t, h, organizer(f), http.MethodGet, eventsURL, "", "", 200)
		var page transport.EventsListResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
		require.Len(t, page.Items, 1)
		require.Equal(t, e.ID, page.Items[0].ID)
		require.Equal(t, status, page.Items[0].Status)
	}
	f.Exec(t, `UPDATE events SET status='draft' WHERE id=$1`, e.ID)
	f.Session(t, e.ID)
	httpCall(t, h, organizer(f), http.MethodDelete, path, "", "", 409)
	httpCall(t, h, organizer(f), http.MethodGet, path, "", "", 200)
}

func TestEventsHTTPPostgresInvalidRequests(t *testing.T) {
	f := testutil.New(t)
	h := integrationRouter(f)
	identity := organizer(f)
	e := f.Event(t, f.OrgID, "Keep unchanged")
	path := eventsURL + "/" + e.ID.String()
	for _, method := range []string{http.MethodGet, http.MethodPatch, http.MethodDelete} {
		for _, id := range []string{"bad-id", uuid.Nil.String()} {
			httpCall(t, h, identity, method, eventsURL+"/"+id, `{"name":"Valid"}`, "application/json", 400)
		}
		httpCall(t, h, identity, method, eventsURL+"/"+uuid.New().String(), `{"name":"Valid"}`, "application/json", 404)
	}
	for _, query := range []string{"limit=0", "limit=-1", "limit=101", "offset=-1", "limit=abc", "offset=abc", "limit=", "offset=", "limit=1&limit=2", "limit=99999999999999999999999999999999999999"} {
		httpCall(t, h, identity, http.MethodGet, eventsURL+"?"+query, "", "", 400)
	}
	for _, method := range []string{http.MethodPost, http.MethodPatch} {
		url := eventsURL
		if method == http.MethodPatch {
			url = path
		}
		for _, body := range []string{"", `{}`, `null`, `[]`, `{"name":123}`, `{"name":""}`, `{"name":" "}`, `{"name":"Valid","unexpected":true}`, `{"name":"Valid"} {}`, `{"name":"Go\u0000Live"}`, `{"name":"Valid","description":"bad\u0000text"}`, `{"name":"Valid","category":"bad\u0000text"}`} {
			httpCall(t, h, identity, method, url, body, "application/json", 400)
		}
		for _, field := range []struct {
			name string
			size int
		}{{"name", 201}, {"description", 10001}, {"category", 101}} {
			body, _ := json.Marshal(map[string]string{"name": "Valid", field.name: strings.Repeat("я", field.size)})
			httpCall(t, h, identity, method, url, string(body), "application/json", 400)
		}
		for _, contentType := range []string{"", "text/plain", "invalid content type"} {
			httpCall(t, h, identity, method, url, `{"name":"Valid"}`, contentType, 415)
		}
		httpCall(t, h, identity, method, url, `{"name":"Valid","description":"`+strings.Repeat("x", 1<<20)+`"}`, "application/json", 413)
	}
	for _, field := range []string{"name", "description", "category"} {
		httpCall(t, h, identity, http.MethodPatch, path, `{"`+field+`":null}`, "application/json", 400)
	}
	w := httpCall(t, h, identity, http.MethodGet, path, "", "", 200)
	var unchanged transport.EventResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &unchanged))
	require.Equal(t, e.Title, unchanged.Title)
	require.Equal(t, e.Description, unchanged.Description)
	require.Equal(t, e.UpdatedAt, unchanged.UpdatedAt)
	// Successful boundary input: lengths count characters rather than UTF-8 bytes.
	body, _ := json.Marshal(map[string]string{"name": strings.Repeat("я", 200), "description": strings.Repeat("д", 10000), "category": strings.Repeat("к", 100)})
	httpCall(t, h, identity, http.MethodPost, eventsURL, string(body), "application/json", 201)
}
