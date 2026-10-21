package ticket_type_service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *TicketTypeService) CreateTicketType(
	ctx context.Context,
	eventID uuid.UUID,
	sessionID uuid.UUID,
	organizerID uuid.UUID,
	userID uuid.UUID,
	name string,
	price domain.Money,
	capacity int,
) (domain.TicketType, error) {
	if eventID == uuid.Nil || sessionID == uuid.Nil || organizerID == uuid.Nil || userID == uuid.Nil {
		return domain.TicketType{}, fmt.Errorf("values must be not zero: %w", core_errors.ErrInvalidArgument)
	}
	ticketType := domain.NewTicketType(
		sessionID,
		name,
		price,
		capacity,
	)

	ticketType.CreatedAt = time.Now().UTC()

	if err := ticketType.Validate(); err != nil {
		return domain.TicketType{}, fmt.Errorf("validate ticket type error: %w", err)
	}

	response, err := s.ticketTypeRepo.CreateTicketType(ctx, ticketType, eventID, organizerID, userID)
	if err != nil {
		return domain.TicketType{}, fmt.Errorf("create ticket type error: %w", err)
	}

	return response, nil
}
