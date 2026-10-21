package ticket_type_transport

import (
	"context"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
)

type HTTPTicketType struct {
	ticketTypeService TicketTypeService
}

type TicketTypeService interface {
	CreateTicketType(
		ctx context.Context,
		eventID uuid.UUID,
		sessionID uuid.UUID,
		organizerID uuid.UUID,
		userID uuid.UUID,
		name string,
		price domain.Money,
		capacity int,
	) (domain.TicketType, error)
	GetTicketType(
		ctx context.Context,
		ticketTypeID uuid.UUID,
		eventID uuid.UUID,
		sessionID uuid.UUID,
		organizerID uuid.UUID,
		userID uuid.UUID,
	) (domain.TicketType, error)

	GetTicketTypes(
		ctx context.Context,
		limit *int,
		offset *int,
		eventID uuid.UUID,
		sessionID uuid.UUID,
		organizerID uuid.UUID,
		userID uuid.UUID,
	) ([]domain.TicketType, error)
}

func NewTicketTypeService(
	ticketTypeService TicketTypeService,
) *HTTPTicketType {
	return &HTTPTicketType{
		ticketTypeService: ticketTypeService,
	}
}
