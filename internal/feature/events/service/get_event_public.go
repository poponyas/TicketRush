package events_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	core_logger "github.com/poponyas/TicketRush/internal/core/logger"
	"go.uber.org/zap"
)

func (s *EventsService) GetPublicEvent(ctx context.Context, eventID uuid.UUID) (domain.Event, error) {
	if eventID == uuid.Nil {
		return domain.Event{}, fmt.Errorf("event ID is required: %w", coreerrors.ErrInvalidArgument)
	}

	if s.eventCache != nil {
		event, found, err := s.eventCache.Get(ctx, eventID)
		if err == nil && found {
			return event, nil
		}

		if err != nil {
			core_logger.FromContext(ctx).Warn(
				"get event cache failed",
				zap.Error(err),
			)
		}

	}
	event, err := s.eventsRepository.GetPublicEvent(ctx, eventID)
	if err != nil {
		return domain.Event{}, fmt.Errorf("get public event: %w", err)
	}

	if s.eventCache != nil {
		if err := s.eventCache.Set(ctx, event); err != nil {
			core_logger.FromContext(ctx).Warn(
				"set event cache failed",
				zap.Error(err),
			)
		}
	}
	return event, nil
}
