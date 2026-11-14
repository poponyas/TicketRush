package account_repository_test

import (
	"context"
	"encoding/json/v2"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	"github.com/poponyas/TicketRush/internal/feature/account"
	repository "github.com/poponyas/TicketRush/internal/feature/account/repository"
	service "github.com/poponyas/TicketRush/internal/feature/account/service"
	transport "github.com/poponyas/TicketRush/internal/feature/account/transport"
	"github.com/poponyas/TicketRush/internal/feature/events/testutil"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOwnAccountHTTPPostgres(t *testing.T) {
	f := testutil.New(t)
	repo := repository.New(&postgres.Pool{Pool: f.Pool, OpTimeout: 5 * time.Second})
	h := transport.New(service.New(repo))
	event := f.Event(t, f.OrgID, "Account event")
	session := f.Session(t, event.ID)
	tariff, booking, otherBooking := uuid.New(), uuid.New(), uuid.New()
	f.Exec(t, `INSERT INTO ticket_types(id,session_id,name,price,currency,capacity,available) VALUES($1,$2,'Regular',999,'RUB',10,10)`, tariff, session)
	for _, b := range []struct{ id, user uuid.UUID }{{booking, f.UserID}, {otherBooking, f.OtherUserID}} {
		f.Exec(t, `INSERT INTO booking(id,user_id,session_id,status,expires_at) VALUES($1,$2,$3,'held',now()+interval '15 minutes')`, b.id, b.user, session)
		f.Exec(t, `INSERT INTO booking_item(id,booking_id,ticket_type_id,quantity,unit_price,currency) VALUES($1,$2,$3,2,12.34,'RUB')`, uuid.New(), b.id, tariff)
	}
	request := func(path string, user uuid.UUID, handler http.HandlerFunc) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		if user != uuid.Nil {
			r = r.WithContext(auth.WithIdentity(r.Context(), auth.Identity{UserID: user}))
		}
		w := httptest.NewRecorder()
		handler(w, r)
		return w
	}
	for _, user := range []uuid.UUID{f.UserID, f.OtherUserID} {
		w := request("/api/v1/me/bookings?limit=1&offset=0&user_id="+f.OtherUserID.String(), user, h.Bookings)
		require.Equal(t, 200, w.Code, w.Body.String())
		require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
		var page account.BookingPage
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
		require.Len(t, page.Items, 1)
		expected := booking
		if user == f.OtherUserID {
			expected = otherBooking
		}
		require.Equal(t, expected, page.Items[0].ID)
		require.Equal(t, "24.68", page.Items[0].Total) // Snapshot, not the current tariff's 999.
		require.EqualValues(t, 2, page.Items[0].Quantity)
		require.Equal(t, event.ID, page.Items[0].EventID)
	}
	for _, path := range []string{"/api/v1/me/bookings?limit=0", "/api/v1/me/bookings?limit=101", "/api/v1/me/bookings?offset=-1", "/api/v1/me/bookings?offset=oops"} {
		require.Equal(t, 400, request(path, f.UserID, h.Bookings).Code)
	}
	require.Equal(t, 401, request("/api/v1/me", uuid.Nil, h.Profile).Code)
	require.Equal(t, 401, request("/api/v1/me/bookings", uuid.Nil, h.Bookings).Code)
	w := request("/api/v1/me", f.UserID, h.Profile)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.NotContains(t, w.Body.String(), "password_hash")
	var profile account.Profile
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &profile))
	require.Len(t, profile.Organizations, 2)
	require.False(t, profile.VenueOwner)
	f.Exec(t, `INSERT INTO user_roles(user_id,role) VALUES($1,'ownerVenue')`, f.UserID)
	w = request("/api/v1/me", f.UserID, h.Profile)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &profile))
	require.True(t, profile.VenueOwner)
	f.Exec(t, `DELETE FROM organizer_memberships WHERE user_id=$1`, f.UserID)
	w = request("/api/v1/me", f.UserID, h.Profile)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &profile))
	require.Empty(t, profile.Organizations)
	_, err := repo.Profile(context.Background(), uuid.New())
	require.Error(t, err)
}
