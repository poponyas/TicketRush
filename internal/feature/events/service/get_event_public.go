package events_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *EventsService) GetPublicEvent(ctx context.Context, eventID uuid.UUID) (domain.Event, error) {
	if eventID == uuid.Nil {
		return domain.Event{}, fmt.Errorf("event ID is required: %w", coreerrors.ErrInvalidArgument)
	}
	event, err := s.eventsRepository.GetPublicEvent(ctx, eventID)
	if err != nil {
		return domain.Event{}, fmt.Errorf("get public event: %w", err)
	}
	return event, nil
}
