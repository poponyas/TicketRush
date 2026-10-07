package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

type Event struct {
	ID               uuid.UUID `json:"id"`
	Title            string    `json:"name"`
	TotalTickets     int       `json:"total_tickets"`
	AvailableTickets int       `json:"available_tickets"`
	Created_at       time.Time `json:"created_at"`
}

func NewEvent(title string, totalTickets int) Event {
	newID := uuid.New()
	return Event{
		ID:               newID,
		Title:            title,
		TotalTickets:     totalTickets,
		AvailableTickets: totalTickets,
		Created_at:       time.Now(),
	}
}

func (e Event) Validate() error {
	e.Title = strings.TrimSpace(e.Title)
	if e.Title == "" {
		return core_errors.ErrInvalidArgument
	}

	if len([]rune(e.Title)) > 200 {
		return core_errors.ErrInvalidArgument
	}

	if e.TotalTickets < 0 {
		return core_errors.ErrInvalidArgument
	}
	return nil
}
