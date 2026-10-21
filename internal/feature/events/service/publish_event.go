package events_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *EventsService) PublishEvent(ctx context.Context, eventID uuid.UUID, organizerID uuid.UUID, userID uuid.UUID) (domain.Event, error) {
	if eventID == uuid.Nil || organizerID == uuid.Nil || userID == uuid.Nil {
		return domain.Event{}, core_errors.ErrInvalidArgument
	}

	response, err := s.eventsRepository.PublishEvent(ctx, eventID, organizerID, userID)
	if err != nil {
		return domain.Event{}, fmt.Errorf("publish event error: %w", err)
	}

	return response, nil
}
