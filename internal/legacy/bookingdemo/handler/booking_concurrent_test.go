package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/legacy/bookingdemo/handler"
	domain "github.com/poponyas/TicketRush/internal/legacy/bookingdemo/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateBookingConcurrentRequests(t *testing.T) {
	const (
		totalTickets = 20
		requestCount = 100
	)

	event := domain.NewEvent("Go Live", totalTickets)
	h := handler.New([]domain.Event{event})
	router := chi.NewRouter()
	router.Get("/events/{id}", h.GetEvent)
	router.Post("/events/{id}/bookings", h.CreateBooking)
	router.Get("/bookings/{id}", h.GetBooking)

	type response struct {
		userID string
		status int
		body   []byte
	}

	// Буфер вмещает все ответы: goroutines смогут завершиться до их чтения.
	responses := make(chan response, requestCount)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(requestCount)

	for i := range requestCount {
		go func(i int) {
			defer wg.Done()
			<-start

			userID := fmt.Sprintf("user-%d", i)
			body := fmt.Sprintf(`{"user_id":%q,"quantity":1}`, userID)
			request := httptest.NewRequest(
				http.MethodPost,
				"/events/"+event.ID.String()+"/bookings",
				strings.NewReader(body),
			)
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			// Проверки выполняются в goroutine теста, здесь только сбор ответов.
			responses <- response{
				userID: userID,
				status: recorder.Code,
				body:   recorder.Body.Bytes(),
			}
		}(i)
	}

	// Одновременно разрешаем всем goroutines начать отправку запросов.
	close(start)
	wg.Wait()
	close(responses)

	created, conflicts := 0, 0
	bookings := make(map[uuid.UUID]domain.Booking)
	for response := range responses {
		switch response.status {
		case http.StatusCreated:
			created++
			var booking domain.Booking
			require.NoError(t, json.Unmarshal(response.body, &booking))
			require.NotEqual(t, uuid.Nil, booking.ID)
			require.NotContains(t, bookings, booking.ID, "ID брони повторяется")
			assert.Equal(t, event.ID, booking.EventID)
			assert.Equal(t, response.userID, booking.UserID)
			assert.Equal(t, 1, booking.Quantity)
			assert.Equal(t, "reserved", booking.Status)
			bookings[booking.ID] = booking

		case http.StatusConflict:
			conflicts++

		default:
			t.Errorf("Неожиданный статус %d: %s", response.status, response.body)
		}
	}

	assert.Equal(t, totalTickets, created)
	assert.Equal(t, requestCount-totalTickets, conflicts)
	require.Len(t, bookings, totalTickets)

	// Каждая успешная бронь должна существовать в хранилище.
	bookedTickets := 0
	for id, expected := range bookings {
		request := httptest.NewRequest(http.MethodGet, "/bookings/"+id.String(), nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code)

		var actual domain.Booking
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &actual))
		assert.Equal(t, expected, actual)
		bookedTickets += actual.Quantity
	}
	assert.Equal(t, totalTickets, bookedTickets)

	request := httptest.NewRequest(http.MethodGet, "/events/"+event.ID.String(), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var updatedEvent domain.Event
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &updatedEvent))
	assert.Equal(t, 0, updatedEvent.AvailableTickets)
	assert.Equal(t, totalTickets, updatedEvent.TotalTickets)

	t.Logf("Создано броней: %d; отказов: %d; остаток: %d", created, conflicts, updatedEvent.AvailableTickets)
}
