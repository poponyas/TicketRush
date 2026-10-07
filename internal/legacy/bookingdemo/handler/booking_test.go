package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/legacy/bookingdemo/handler"
	domain "github.com/poponyas/TicketRush/internal/legacy/bookingdemo/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateBookingDecreasesAvailableTickets(t *testing.T) {
	event := domain.NewEvent("Go Live", 20)
	h := handler.New([]domain.Event{event})

	router := chi.NewRouter()
	router.Get("/events/{id}", h.GetEvent)
	router.Post("/events/{id}/bookings", h.CreateBooking)

	body := `{"user_id":"user-1","quantity":2}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/events/"+event.ID.String()+"/bookings",
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	// 3. Проверяем ответ с бронью.
	require.Equal(t, http.StatusCreated, response.Code)

	var booking domain.Booking
	require.NoError(
		t,
		json.Unmarshal(response.Body.Bytes(), &booking),
	)

	assert.NotEqual(t, uuid.Nil, booking.ID)
	assert.Equal(t, event.ID, booking.EventID)
	assert.Equal(t, "user-1", booking.UserID)
	assert.Equal(t, 2, booking.Quantity)
	assert.Equal(t, "reserved", booking.Status)

	// 4. Через GET проверяем фактический остаток.
	getRequest := httptest.NewRequest(
		http.MethodGet,
		"/events/"+event.ID.String(),
		nil,
	)

	getResponse := httptest.NewRecorder()
	router.ServeHTTP(getResponse, getRequest)

	require.Equal(t, http.StatusOK, getResponse.Code)

	var updatedEvent domain.Event
	require.NoError(
		t,
		json.Unmarshal(getResponse.Body.Bytes(), &updatedEvent),
	)

	assert.Equal(t, 18, updatedEvent.AvailableTickets)
	assert.Equal(t, 20, updatedEvent.TotalTickets)
}

func TestBookingTicketsMoreThanItsAvailable(t *testing.T) {
	event := domain.NewEvent("Go Live", 20)
	h := handler.New([]domain.Event{event})

	router := chi.NewRouter()
	router.Get("/events/{id}", h.GetEvent)
	router.Post("/events/{id}/bookings", h.CreateBooking)

	body := `{"user_id":"user-1","quantity":21}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/events/"+event.ID.String()+"/bookings",
		strings.NewReader(body),
	)

	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusConflict, response.Code)

	getRequest := httptest.NewRequest(
		http.MethodGet,
		"/events/"+event.ID.String(),
		nil,
	)

	getResponse := httptest.NewRecorder()
	router.ServeHTTP(getResponse, getRequest)

	require.Equal(t, http.StatusOK, getResponse.Code)

	var updatedEvent domain.Event
	require.NoError(
		t,
		json.Unmarshal(getResponse.Body.Bytes(), &updatedEvent),
	)

	assert.Equal(t, 20, updatedEvent.AvailableTickets)
}
