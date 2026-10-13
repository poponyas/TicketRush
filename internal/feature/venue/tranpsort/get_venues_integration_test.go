package venue_http_tranpsort_test

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	service "github.com/poponyas/TicketRush/internal/feature/venue/service"
	"github.com/poponyas/TicketRush/internal/feature/venue/testutil"
	transport "github.com/poponyas/TicketRush/internal/feature/venue/tranpsort"
	"github.com/stretchr/testify/require"
)

func TestVenueHTTPReadsPostgres(t *testing.T) {
	pool, repo := testutil.Database(t)
	h := transport.NewHTTPVenueTransport(service.NewVenueService(repo))
	r := chi.NewRouter()
	r.Get("/api/v1/venues", h.GetVenues)
	r.Get("/api/v1/venues/{venue_id}", h.GetVenue)
	r.With(auth.RequireVenueOwner).Post("/api/v1/venues", h.CreateVenue)
	call := func(method, path, body string, identity *auth.Identity, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if identity != nil {
			req = req.WithContext(auth.WithIdentity(req.Context(), *identity))
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, status, w.Code, w.Body.String())
		require.Equal(t, "application/json", w.Header().Get("Content-Type"))
		return w
	}
	// Reading the shared catalog requires neither ownerVenue nor an identity.
	w := call(http.MethodGet, "/api/v1/venues", "", nil, 200)
	require.JSONEq(t, `{"items":[],"limit":20,"offset":0}`, w.Body.String())
	owner := &auth.Identity{UserID: uuid.New(), Roles: []domain.Role{domain.RoleVenueOwner}}
	w = call(http.MethodPost, "/api/v1/venues", `{"name":" Hall ","city":" Moscow ","address":" Address "}`, owner, 201)
	var created transport.VenueResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	path := "/api/v1/venues/" + created.ID.String()
	w = call(http.MethodGet, path, "", nil, 200)
	var fetched transport.VenueResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &fetched))
	require.Equal(t, created, fetched)
	require.Equal(t, "Hall", fetched.Name)
	require.Equal(t, "Moscow", fetched.City)
	require.Equal(t, "Address", fetched.Address)
	w = call(http.MethodGet, "/api/v1/venues?limit=1&offset=0", "", nil, 200)
	var page transport.VenuesListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Len(t, page.Items, 1)
	require.Equal(t, created, page.Items[0])
	require.Equal(t, 1, page.Limit)
	w = call(http.MethodGet, "/api/v1/venues?offset=1", "", nil, 200)
	require.JSONEq(t, `{"items":[],"limit":20,"offset":1}`, w.Body.String())
	for _, id := range []string{"invalid", uuid.Nil.String()} {
		call(http.MethodGet, "/api/v1/venues/"+id, "", nil, 400)
	}
	call(http.MethodGet, "/api/v1/venues/"+uuid.New().String(), "", nil, 404)
	for _, query := range []string{"limit=0", "limit=-1", "limit=101", "offset=-1", "limit=abc", "offset=abc", "limit=", "limit=1&limit=2"} {
		call(http.MethodGet, "/api/v1/venues?"+query, "", nil, 400)
	}
	call(http.MethodPost, "/api/v1/venues", `{"name":"Hall","city":"City","address":"Address"}`, nil, 401)
	// A database failure must become 500 without exposing SQL details.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := pool.Exec(ctx, `DROP TABLE venues`)
	require.NoError(t, err)
	for _, url := range []string{path, "/api/v1/venues"} {
		w = call(http.MethodGet, url, "", nil, 500)
		require.NotContains(t, w.Body.String(), "SQLSTATE")
		require.NotContains(t, w.Body.String(), "does not exist")
	}
}
