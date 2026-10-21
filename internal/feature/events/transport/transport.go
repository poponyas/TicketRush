package events_transport

import (
	"context"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	service "github.com/poponyas/TicketRush/internal/feature/events/service"
)

type EventsHTTPTransport struct{ eventService EventsService }
type EventsService interface {
	CreateEvent(ctx context.Context, input service.CreateEventInput) (domain.Event, error)
	GetEvent(ctx context.Context, id uuid.UUID, organizerID uuid.UUID, userID uuid.UUID) (domain.Event, error)
	GetEvents(ctx context.Context, limit *int, offset *int, organizerID uuid.UUID, userID uuid.UUID) (service.EventsPage, error)
	UpdateEvent(ctx context.Context, input service.UpdateEventInput, eventID uuid.UUID, organizerID uuid.UUID, userID uuid.UUID) (domain.Event, error)
	DeleteEvent(ctx context.Context, eventID uuid.UUID, organizerID uuid.UUID, userID uuid.UUID) error
	PublishEvent(ctx context.Context, eventID uuid.UUID, organizerID uuid.UUID, userID uuid.UUID) (domain.Event, error)
}

func NewEventsHTTPTransport(service EventsService) *EventsHTTPTransport {
	return &EventsHTTPTransport{eventService: service}
}
