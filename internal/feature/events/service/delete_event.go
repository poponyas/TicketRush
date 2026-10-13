package events_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *EventsService) DeleteEvent(ctx context.Context, eventID, organizerID, userID uuid.UUID) error {
	if eventID == uuid.Nil || organizerID == uuid.Nil || userID == uuid.Nil {
		return fmt.Errorf("%w: event, organizer and user IDs are required", coreerrors.ErrInvalidArgument)
	}
	// Status and access must be checked together with deletion in the repository.
	if err := s.eventsRepository.DeleteEvent(ctx, eventID, organizerID, userID); err != nil {
		return fmt.Errorf("delete event: %w", err)
	}
	return nil
}
