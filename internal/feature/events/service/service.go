package events_service

import (
	"context"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
)

type EventsService struct{ eventsRepository EventsRepository }
type CreateEventInput struct {
	UserID      uuid.UUID
	OrganizerID uuid.UUID
	Title       string
	Description string
	Category    string
}
type EventsRepository interface {
	CreateEvent(ctx context.Context, event domain.Event, actorID uuid.UUID) (domain.Event, error)
}

func NewEventsService(repo EventsRepository) *EventsService {
	return &EventsService{eventsRepository: repo}
}
