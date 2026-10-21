package session_transport_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	"github.com/poponyas/TicketRush/internal/feature/events/testutil"
	rateRepo "github.com/poponyas/TicketRush/internal/feature/rate/repository"
	rateService "github.com/poponyas/TicketRush/internal/feature/rate/service"
	rateHTTP "github.com/poponyas/TicketRush/internal/feature/rate/transport"
	sessionRepo "github.com/poponyas/TicketRush/internal/feature/session/repository"
	sessionService "github.com/poponyas/TicketRush/internal/feature/session/service"
	sessionHTTP "github.com/poponyas/TicketRush/internal/feature/session/transport"
	"github.com/stretchr/testify/require"
)

func TestPublicCatalogPostgres(t *testing.T) {
	f := testutil.New(t)
	event := f.Event(t, f.OrgID, "Public event")
	other := f.Event(t, f.OtherOrgID, "Other public event")
	draft := f.Event(t, f.OrgID, "Hidden draft")
	archived := f.Event(t, f.OrgID, "Hidden archive")
	empty := f.Event(t, f.OrgID, "Empty public event")
	f.Exec(t, `UPDATE events SET status='published' WHERE id IN ($1,$2,$3)`, event.ID, other.ID, empty.ID)
	f.Exec(t, `UPDATE events SET status='archived' WHERE id=$1`, archived.ID)
	now := time.Now().UTC().Truncate(time.Second)
	addSession := func(eventID uuid.UUID, status domain.SessionStatus, start time.Time) uuid.UUID {
		t.Helper()
		id := f.Session(t, eventID)
		f.Exec(t, `UPDATE sessions SET status=$1,starts_at=$2,sales_start_at=$3,sales_end_at=$4 WHERE id=$5`, status, start, start.Add(-48*time.Hour), start.Add(-time.Hour), id)
		return id
	}
	first := addSession(event.ID, domain.SessionScheduled, now.Add(24*time.Hour))
	ties := []uuid.UUID{
		addSession(event.ID, domain.SessionScheduled, now.Add(48*time.Hour)),
		addSession(event.ID, domain.SessionScheduled, now.Add(48*time.Hour)),
	}
	sort.Slice(ties, func(i, j int) bool { return ties[i].String() < ties[j].String() })
	foreign := addSession(other.ID, domain.SessionScheduled, now.Add(24*time.Hour))
	hidden := []struct{ event, session uuid.UUID }{
		{draft.ID, addSession(draft.ID, domain.SessionScheduled, now.Add(24*time.Hour))},
		{archived.ID, addSession(archived.ID, domain.SessionScheduled, now.Add(24*time.Hour))},
		{event.ID, addSession(event.ID, domain.SessionDraft, now.Add(24*time.Hour))},
		{event.ID, addSession(event.ID, domain.SessionCancelled, now.Add(24*time.Hour))},
		{event.ID, addSession(event.ID, domain.SessionCompleted, now.Add(24*time.Hour))},
		{event.ID, addSession(event.ID, domain.SessionScheduled, now.Add(-24*time.Hour))},
	}
	addTariff := func(t *testing.T, sessionID, id uuid.UUID, soldOut bool) {
		t.Helper()
		available, sold := 10, 0
		if soldOut {
			available, sold = 0, 10
		}
		f.Exec(t, `INSERT INTO ticket_types(id,session_id,name,price,currency,capacity,available,held,sold,created_at)
   VALUES($1,$2,'Standard',1500.25,'RUB',10,$3,0,$4,$5)`, id, sessionID, available, sold, now)
	}
	low := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	high := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	addTariff(t, first, low, false)
	addTariff(t, first, high, true)
	foreignTariff := uuid.New()
	addTariff(t, foreign, foreignTariff, false)
	wrapper := &postgres.Pool{Pool: f.Pool, OpTimeout: 5 * time.Second}
	sh := sessionHTTP.NewHTTPSessionTransport(sessionService.NewSessionService(sessionRepo.NewSessionRepository(wrapper)))
	th := rateHTTP.NewTicketTypeService(rateService.NewTicketTypeService(rateRepo.NewTicketTypePool(wrapper)))
	router := chi.NewRouter()
	router.Get("/api/v1/events/{event_id}/sessions", sh.GetPublicSessions)
	router.Get("/api/v1/events/{event_id}/sessions/{session_id}", sh.GetPublicSession)
	router.Get("/api/v1/events/{event_id}/sessions/{session_id}/ticket-types", th.GetPublicTicketTypes)
	router.Get("/api/v1/events/{event_id}/sessions/{session_id}/ticket-types/{ticket_type_id}", th.GetPublicTicketType)
	eventURL := func(id uuid.UUID) string { return "/api/v1/events/" + id.String() + "/sessions" }
	sessionURL := func(e, s uuid.UUID) string { return eventURL(e) + "/" + s.String() }
	get := func(t *testing.T, url string, code int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
		require.Equal(t, code, w.Code, "GET %s: %s", url, w.Body.String())
		return w
	}

	t.Run("public session list and pagination", func(t *testing.T) {
		w := get(t, eventURL(event.ID)+"?limit=2", 200)
		var page sessionHTTP.SessionsListResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
		require.Equal(t, 2, page.Limit)
		require.Zero(t, page.Offset)
		require.Len(t, page.Items, 2)
		require.Equal(t, first, page.Items[0].ID)
		require.Equal(t, ties[0], page.Items[1].ID)
		w = get(t, eventURL(event.ID)+"?limit=2&offset=2", 200)
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
		require.Equal(t, 2, page.Offset)
		require.Len(t, page.Items, 1)
		require.Equal(t, ties[1], page.Items[0].ID)
		require.Contains(t, get(t, eventURL(event.ID)+"?offset=3", 200).Body.String(), `"items":[]`)
		require.Contains(t, get(t, eventURL(empty.ID), 200).Body.String(), `"items":[]`)
		for _, id := range []uuid.UUID{draft.ID, archived.ID, uuid.New()} {
			get(t, eventURL(id), 404)
		}
	})
	t.Run("session detail and nesting", func(t *testing.T) {
		w := get(t, sessionURL(event.ID, first), 200)
		var body sessionHTTP.SessionResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Equal(t, first, body.ID)
		require.Equal(t, event.ID, body.EventID)
		require.Equal(t, domain.SessionScheduled, body.Status)
		get(t, sessionURL(event.ID, foreign), 404)
		get(t, sessionURL(other.ID, first), 404)
		get(t, sessionURL(event.ID, uuid.New()), 404)
	})
	t.Run("tariff pages price and sold out", func(t *testing.T) {
		url := sessionURL(event.ID, first) + "/ticket-types"
		var page rateHTTP.TicketTypesListResponse
		require.NoError(t, json.Unmarshal(get(t, url, 200).Body.Bytes(), &page))
		require.Equal(t, 20, page.Limit)
		require.Len(t, page.Items, 2)
		require.Equal(t, low, page.Items[0].ID)
		require.Equal(t, high, page.Items[1].ID)
		require.Zero(t, page.Items[1].Available)
		require.Equal(t, "1500.25", page.Items[0].Price.Amount.String())
		require.Equal(t, "RUB", page.Items[0].Price.Currency)
		require.NoError(t, json.Unmarshal(get(t, url+"?limit=1&offset=1", 200).Body.Bytes(), &page))
		require.Len(t, page.Items, 1)
		require.Equal(t, high, page.Items[0].ID)
		require.Equal(t, 1, page.Offset)
		require.Contains(t, get(t, url+"?offset=2", 200).Body.String(), `"items":[]`)
		require.Contains(t, get(t, sessionURL(event.ID, ties[0])+"/ticket-types", 200).Body.String(), `"items":[]`)
		var item rateHTTP.TicketTypeResponse
		require.NoError(t, json.Unmarshal(get(t, url+"/"+low.String(), 200).Body.Bytes(), &item))
		require.Equal(t, low, item.ID)
		require.Equal(t, first, item.SessionID)
		require.Equal(t, "1500.25", item.Price.Amount.String())
		get(t, url+"/"+foreignTariff.String(), 404)
		get(t, url+"/"+uuid.New().String(), 404)
		get(t, sessionURL(event.ID, foreign)+"/ticket-types", 404)
		get(t, sessionURL(event.ID, ties[0])+"/ticket-types/"+low.String(), 404)
		get(t, sessionURL(event.ID, uuid.New())+"/ticket-types", 404)
	})
	t.Run("hidden sessions also hide tariffs", func(t *testing.T) {
		for _, row := range hidden {
			id := uuid.New()
			addTariff(t, row.session, id, false)
			url := sessionURL(row.event, row.session)
			get(t, url, 404)
			get(t, url+"/ticket-types", 404)
			get(t, url+"/ticket-types/"+id.String(), 404)
		}
	})
	t.Run("bad parameters", func(t *testing.T) {
		for _, url := range []string{eventURL(event.ID), sessionURL(event.ID, first) + "/ticket-types"} {
			for _, query := range []string{"?limit=0", "?limit=101", "?offset=-1", "?limit=abc", "?offset=", "?limit=1&limit=2"} {
				get(t, url+query, 400)
			}
		}
		get(t, "/api/v1/events/not-a-uuid/sessions", 400)
		get(t, eventURL(uuid.Nil), 400)
		get(t, eventURL(event.ID)+"/not-a-uuid", 400)
		get(t, sessionURL(event.ID, uuid.Nil), 400)
		get(t, sessionURL(event.ID, first)+"/ticket-types/not-a-uuid", 400)
		get(t, sessionURL(event.ID, first)+"/ticket-types/"+uuid.Nil.String(), 400)
	})
	t.Run("closed sales remain visible", func(t *testing.T) {
		f.Exec(t, `UPDATE sessions SET sales_start_at=$1,sales_end_at=$2 WHERE id=$3`, now.Add(-2*time.Hour), now.Add(-time.Hour), first)
		get(t, sessionURL(event.ID, first), 200)
		get(t, sessionURL(event.ID, first)+"/ticket-types", 200)
	})
}
