package events_transport

import (
	"context"
	"github.com/poponyas/TicketRush/internal/core/domain"
	service "github.com/poponyas/TicketRush/internal/feature/events/service"
)

type EventsHTTPTransport struct{ eventService EventsService }
type EventsService interface {
	CreateEvent(ctx context.Context, input service.CreateEventInput) (domain.Event, error)
}

func NewEventsHTTPTransport(service EventsService) *EventsHTTPTransport {
	return &EventsHTTPTransport{eventService: service}
}
