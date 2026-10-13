package events_service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *EventsService) UpdateEvent(
	ctx context.Context,
	input UpdateEventInput,
	eventID uuid.UUID,
	organizerID uuid.UUID,
	userID uuid.UUID,
) (domain.Event, error) {
	if eventID == uuid.Nil || organizerID == uuid.Nil || userID == uuid.Nil {
		return domain.Event{}, fmt.Errorf("%w: event, organizer and user IDs are required", coreerrors.ErrInvalidArgument)
	}
	if input.Name == nil && input.Description == nil && input.Category == nil {
		return domain.Event{}, fmt.Errorf("%w: at least one field must be provided", coreerrors.ErrInvalidArgument)
	}

	event, err := s.eventsRepository.GetEvent(ctx, eventID, organizerID, userID)
	if err != nil {
		return domain.Event{}, fmt.Errorf("get event: %w", err)
	}
	if event.Status != domain.EventDraft {
		return domain.Event{}, fmt.Errorf("%w: only draft events can be updated", coreerrors.ErrConflict)
	}

	if input.Name != nil {
		event.Title = strings.TrimSpace(*input.Name)
	}
	if input.Description != nil {
		event.Description = strings.TrimSpace(*input.Description)
	}
	if input.Category != nil {
		event.Category = strings.TrimSpace(*input.Category)
	}
	if err := event.Validate(); err != nil {
		return domain.Event{}, fmt.Errorf("validate updated event: %w", err)
	}

	updated, err := s.eventsRepository.UpdateEvent(ctx, event, userID)
	if err != nil {
		return domain.Event{}, fmt.Errorf("update event: %w", err)
	}
	return updated, nil
}
