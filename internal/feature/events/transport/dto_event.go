package events_transport

import (
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
)

type EventCreateRequest struct {
	Title       string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}
type EventResponse struct {
	ID          uuid.UUID          `json:"id"`
	OrganizerID uuid.UUID          `json:"organizer_id"`
	Title       string             `json:"name"`
	Description string             `json:"description"`
	Category    string             `json:"category"`
	PosterKey   string             `json:"poster_key,omitempty"`
	PosterURL   string             `json:"poster_url,omitempty"`
	Status      domain.EventStatus `json:"status"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

type EventsListResponse struct {
	Items  []EventResponse `json:"items"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

func eventResponse(event domain.Event) EventResponse {
	posterURL := ""
	if event.PosterKey != "" {
		posterURL = "/api/v1/events/" + event.ID.String() + "/poster"
	}
	return EventResponse{ID: event.ID, OrganizerID: event.OrganizerID, Title: event.Title,
		Description: event.Description, Category: event.Category, PosterKey: event.PosterKey, PosterURL: posterURL,
		Status: event.Status, CreatedAt: event.CreatedAt, UpdatedAt: event.UpdatedAt}
}

func eventsResponse(events []domain.Event) []EventResponse {
	evntResponse := make([]EventResponse, 0, len(events))
	for _, event := range events {
		ev := eventResponse(event)
		evntResponse = append(evntResponse, ev)
	}

	return evntResponse
}
