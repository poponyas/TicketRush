package events_service

import (
	"context"
	"fmt"

	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *EventsService) GetPublicEvents(
	ctx context.Context,
	limit, offset *int,
) (EventsPage, error) {
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

	events, err := s.eventsRepository.GetPublicEvents(ctx, page.Limit, page.Offset)
	if err != nil {
		return EventsPage{}, fmt.Errorf("get list of events: %w", err)
	}
	page.Events = events
	return page, nil
}
