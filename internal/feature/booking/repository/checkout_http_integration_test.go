package booking_repository_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	"github.com/poponyas/TicketRush/internal/core/security/ticketqr"
	checkoutservice "github.com/poponyas/TicketRush/internal/feature/checkout/service"
	checkouttransport "github.com/poponyas/TicketRush/internal/feature/checkout/transport"
	ticketrepo "github.com/poponyas/TicketRush/internal/feature/tickets/repository"
	ticketservice "github.com/poponyas/TicketRush/internal/feature/tickets/service"
	tickettransport "github.com/poponyas/TicketRush/internal/feature/tickets/transport"
	"github.com/stretchr/testify/require"
	"image/png"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCheckoutAndCheckInHTTPPostgres(t *testing.T) {
	f := newExpirationFixture(t, 5)
	booking := heldBooking(t, f, 2)
	signer, err := ticketqr.New(bytes.Repeat([]byte{1}, 32))
	require.NoError(t, err)
	repo := ticketrepo.New(&postgres.Pool{Pool: f.Pool, OpTimeout: 5 * time.Second})
	tickets := tickettransport.New(ticketservice.New(repo, signer))
	checkout := checkouttransport.New(checkoutservice.New(f.repo, true))
	r := chi.NewRouter()
	r.Post("/bookings/{booking_id}/confirm", checkout.Confirm)
	r.Get("/tickets", tickets.List)
	r.Get("/tickets/{ticket_id}", tickets.Get)
	r.Get("/tickets/{ticket_id}/qr", tickets.QR)
	r.Post("/sessions/{session_id}/check-ins", tickets.CheckIn)
	request := func(user uuid.UUID, method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if user != uuid.Nil {
			req = req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{UserID: user}))
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	require.Equal(t, 401, request(uuid.Nil, "POST", "/bookings/"+booking.ID.String()+"/confirm", "").Code)
	for range 2 {
		w := request(f.UserID, "POST", "/bookings/"+booking.ID.String()+"/confirm", "")
		require.Equal(t, 200, w.Code, w.Body.String())
	}
	list := request(f.UserID, "GET", "/tickets", "")
	require.Equal(t, 200, list.Code)
	var page struct {
		Items []tickettransport.TicketResponse `json:"items"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &page))
	require.Len(t, page.Items, 2)
	id := page.Items[0].ID
	wrong := request(f.OtherUserID, "GET", "/tickets/"+id.String(), "")
	require.Equal(t, 404, wrong.Code)
	require.Equal(t, 400, request(f.UserID, "GET", "/tickets?limit=0", "").Code)
	w := request(f.UserID, "GET", "/tickets/"+id.String(), "")
	require.Equal(t, 200, w.Code)
	var detail tickettransport.TicketResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &detail))
	require.NotEmpty(t, detail.QRPayload)
	w = request(f.UserID, "GET", "/tickets/"+id.String()+"/qr", "")
	require.Equal(t, 200, w.Code)
	_, err = png.Decode(bytes.NewReader(w.Body.Bytes()))
	require.NoError(t, err)
	body, _ := json.Marshal(map[string]string{"token": detail.QRPayload})
	path := "/sessions/" + f.sessionID.String() + "/check-ins"
	require.Equal(t, 404, request(f.OtherUserID, "POST", path, string(body)).Code)
	f.Exec(t, `INSERT INTO organizer_memberships(organizer_id,user_id,role) VALUES($1,$2,'controller')`, f.OrgID, f.OtherUserID)
	// Exactly one simultaneous scan wins.
	var wg sync.WaitGroup
	codes := make(chan int, 8)
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); codes <- request(f.OtherUserID, "POST", path, string(body)).Code }()
	}
	wg.Wait()
	close(codes)
	success, conflict := 0, 0
	for code := range codes {
		if code == 200 {
			success++
		} else if code == 409 {
			conflict++
		} else {
			t.Errorf("unexpected scan code %d", code)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 7, conflict)
	owned, err := repo.Get(context.Background(), id, f.UserID)
	require.NoError(t, err)
	require.NotNil(t, owned.UsedAt)
	require.Equal(t, f.OtherUserID, *owned.CheckedInBy)
	require.Equal(t, 409, request(f.UserID, "GET", "/tickets/"+id.String()+"/qr", "").Code)
	// The controller loses access immediately when the live membership is removed.
	f.Exec(t, `DELETE FROM organizer_memberships WHERE organizer_id=$1 AND user_id=$2`, f.OrgID, f.OtherUserID)
	require.Equal(t, 404, request(f.OtherUserID, "POST", path, string(body)).Code)
}
