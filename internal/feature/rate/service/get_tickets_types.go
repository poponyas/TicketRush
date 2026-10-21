package ticket_type_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *TicketTypeService) GetTicketTypes(
	ctx context.Context,
	limit *int,
	offset *int,
	eventID uuid.UUID,
	sessionID uuid.UUID,
	organizerID uuid.UUID,
	userID uuid.UUID,
) ([]domain.TicketType, error) {
	if eventID == uuid.Nil || sessionID == uuid.Nil || userID == uuid.Nil || organizerID == uuid.Nil {
		return nil, fmt.Errorf("values must be not zero: %w", core_errors.ErrInvalidArgument)
	}

	pageLimit, pageOffset := 20, 0
	if limit != nil {
		pageLimit = *limit
	}
	if offset != nil {
		pageOffset = *offset
	}
	if pageLimit < 1 || pageLimit > 100 {
		return nil, fmt.Errorf("limit must be between 1 and 100: %w", core_errors.ErrInvalidArgument)
	}
	if pageOffset < 0 {
		return nil, fmt.Errorf("offset must be nonnegative: %w", core_errors.ErrInvalidArgument)
	}

	response, err := s.ticketTypeRepo.GetTicketTypes(ctx, pageLimit, pageOffset, eventID, sessionID, organizerID, userID)
	if err != nil {
		return nil, fmt.Errorf("get ticket type: %w", err)
	}

	return response, nil
}
