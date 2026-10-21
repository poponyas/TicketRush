package ticket_type_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *TicketTypeService) GetTicketType(
	ctx context.Context,
	ticketTypeID uuid.UUID,
	eventID uuid.UUID,
	sessionID uuid.UUID,
	organizerID uuid.UUID,
	userID uuid.UUID,
) (domain.TicketType, error) {
	if ticketTypeID == uuid.Nil || eventID == uuid.Nil || sessionID == uuid.Nil || userID == uuid.Nil || organizerID == uuid.Nil {
		return domain.TicketType{}, fmt.Errorf("values must be not zero: %w", core_errors.ErrInvalidArgument)
	}

	response, err := s.ticketTypeRepo.GetTicketType(ctx, ticketTypeID, eventID, sessionID, organizerID, userID)
	if err != nil {
		return domain.TicketType{}, fmt.Errorf("get ticket type: %w", err)
	}

	return response, nil
}
