package events_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *EventsService) GetEvent(ctx context.Context, id uuid.UUID, organizerID uuid.UUID, userID uuid.UUID) (domain.Event, error) {
	if err := uuid.Validate(id.String()); err != nil {
		return domain.Event{}, fmt.Errorf("validate error: %w", err)
	}

	if id == uuid.Nil || organizerID == uuid.Nil || userID == uuid.Nil {
		return domain.Event{}, core_errors.ErrInvalidArgument
	}

	domainEvent, err := s.eventsRepository.GetEvent(ctx, id, organizerID, userID)
	if err != nil {
		return domain.Event{}, fmt.Errorf("can't getEvent from Service: %w", err)
	}

	return domainEvent, nil

}
