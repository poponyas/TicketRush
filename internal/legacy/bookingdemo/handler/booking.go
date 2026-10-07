package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	domain "github.com/poponyas/TicketRush/internal/legacy/bookingdemo/model"
)

const bookingReserved = "reserved"

type bookingRequest struct {
	UserID   string `json:"user_id"`
	Quantity int    `json:"quantity"`
}

func (h *Handler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var request bookingRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if request.Quantity <= 0 || request.UserID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	event, ok := h.events[id]
	if !ok {
		h.mu.Unlock()
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if request.Quantity > event.AvailableTickets {
		h.mu.Unlock()
		w.WriteHeader(http.StatusConflict)
		return
	}

	event.AvailableTickets -= request.Quantity
	h.events[event.ID] = event
	booking := domain.NewBooking(event.ID, request.UserID, request.Quantity, bookingReserved)
	h.bookings[booking.ID] = booking
	h.mu.Unlock()

	writeJSON(w, http.StatusCreated, booking)
}

func (h *Handler) GetBooking(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Некорректный ID брони", http.StatusBadRequest)
		return
	}

	h.mu.RLock()
	booking, ok := h.bookings[id]
	h.mu.RUnlock()
	if !ok {
		http.Error(w, "Такой брони не существует", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, booking)
}
