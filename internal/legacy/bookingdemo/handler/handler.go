package handler

import (
	"sync"

	"github.com/google/uuid"
	domain "github.com/poponyas/TicketRush/internal/legacy/bookingdemo/model"
)

type Handler struct {
	mu       sync.RWMutex
	events   map[uuid.UUID]domain.Event
	bookings map[uuid.UUID]domain.Booking
}

func New(initialEvents []domain.Event) *Handler {
	h := &Handler{
		events:   make(map[uuid.UUID]domain.Event),
		bookings: make(map[uuid.UUID]domain.Booking),
	}
	for _, event := range initialEvents {
		h.events[event.ID] = event
	}
	return h
}
