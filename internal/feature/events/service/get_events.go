package events_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *EventsService) GetEvents(ctx context.Context, limit *int, offset *int, organizerID uuid.UUID, userID uuid.UUID) (EventsPage, error) {
	if organizerID == uuid.Nil || userID == uuid.Nil {
		return EventsPage{}, core_errors.ErrInvalidArgument
	}
	page := EventsPage{Limit: 20, Offset: 0}
	if limit != nil {
		page.Limit = *limit
	}
	if offset != nil {
		page.Offset = *offset
	}
	if page.Limit < 1 || page.Limit > 100 {
		return EventsPage{}, fmt.Errorf("limit must be between 1 and 100: %w", core_errors.ErrInvalidArgument)
	}
	if page.Offset < 0 {
		return EventsPage{}, fmt.Errorf("offset must be nonnegative: %w", core_errors.ErrInvalidArgument)
	}

	events, err := s.eventsRepository.GetEvents(ctx, page.Limit, page.Offset, organizerID, userID)
	if err != nil {
		return EventsPage{}, fmt.Errorf("get list of events: %w", err)
	}
	page.Events = events
	return page, nil
}
