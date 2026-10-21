package ticket_type_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *TicketTypeService) GetPublicTicketType(ctx context.Context, eventID, sessionID, ticketTypeID uuid.UUID) (domain.TicketType, error) {
	if eventID == uuid.Nil || sessionID == uuid.Nil || ticketTypeID == uuid.Nil {
		return domain.TicketType{}, fmt.Errorf("event, session and ticket type IDs are required: %w", coreerrors.ErrInvalidArgument)
	}
	ticketType, err := s.ticketTypeRepo.GetPublicTicketType(ctx, eventID, sessionID, ticketTypeID)
	if err != nil {
		return domain.TicketType{}, fmt.Errorf("get public ticket type: %w", err)
	}
	return ticketType, nil
}

func (s *TicketTypeService) GetPublicTicketTypes(ctx context.Context, eventID, sessionID uuid.UUID, limit, offset *int) (TicketTypesPage, error) {
	if eventID == uuid.Nil || sessionID == uuid.Nil {
		return TicketTypesPage{}, fmt.Errorf("event and session IDs are required: %w", coreerrors.ErrInvalidArgument)
	}
	page := TicketTypesPage{Limit: 20}
	if limit != nil {
		page.Limit = *limit
	}
	if offset != nil {
		page.Offset = *offset
	}
	if page.Limit < 1 || page.Limit > 100 || page.Offset < 0 {
		return TicketTypesPage{}, fmt.Errorf("limit must be 1..100 and offset nonnegative: %w", coreerrors.ErrInvalidArgument)
	}
	ticketTypes, err := s.ticketTypeRepo.GetPublicTicketTypes(ctx, eventID, sessionID, page.Limit, page.Offset)
	if err != nil {
		return TicketTypesPage{}, fmt.Errorf("list public ticket types: %w", err)
	}
	page.TicketTypes = ticketTypes
	return page, nil
}
