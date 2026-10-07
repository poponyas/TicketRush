package events_service

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *EventsService) CreateEvent(ctx context.Context, input CreateEventInput) (domain.Event, error) {
	if input.UserID == uuid.Nil {
		return domain.Event{}, coreerrors.ErrUnauthenticated
	}
	event := domain.NewEvent(input.OrganizerID, input.Title, input.Description, input.Category)
	if err := event.Validate(); err != nil {
		return domain.Event{}, fmt.Errorf("validate event: %w", err)
	}
	saved, err := s.eventsRepository.CreateEvent(ctx, event, input.UserID)
	if err != nil {
		return domain.Event{}, fmt.Errorf("create event: %w", err)
	}
	return saved, nil
}
