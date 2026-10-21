package events_service

import (
	"context"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
)

type EventsService struct{ eventsRepository EventsRepository }

type EventsPage struct {
	Events []domain.Event
	Limit  int
	Offset int
}
type CreateEventInput struct {
	UserID      uuid.UUID
	OrganizerID uuid.UUID
	Title       string
	Description string
	Category    string
}

type UpdateEventInput struct {
	Name        *string
	Description *string
	Category    *string
}
type EventsRepository interface {
	GetPublicEvent(ctx context.Context, eventID uuid.UUID) (domain.Event, error)
	CreateEvent(ctx context.Context, event domain.Event, actorID uuid.UUID) (domain.Event, error)
	GetEvent(ctx context.Context, id uuid.UUID, organizerID uuid.UUID, userID uuid.UUID) (domain.Event, error)
	GetEvents(ctx context.Context, limit int, offset int, organizerID uuid.UUID, userID uuid.UUID) ([]domain.Event, error)
	UpdateEvent(ctx context.Context, event domain.Event, userID uuid.UUID) (domain.Event, error)
	DeleteEvent(ctx context.Context, eventID uuid.UUID, organizerID uuid.UUID, userID uuid.UUID) error
	PublishEvent(ctx context.Context, eventID uuid.UUID, organizerID uuid.UUID, userID uuid.UUID) (domain.Event, error)
	GetPublicEvents(ctx context.Context, limit, offset int) ([]domain.Event, error)
}

func NewEventsService(repo EventsRepository) *EventsService {
	return &EventsService{eventsRepository: repo}
}
