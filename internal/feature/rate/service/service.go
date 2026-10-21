package ticket_type_service

import (
	"context"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
)

type TicketTypeService struct {
	ticketTypeRepo TicketTypeRepo
}

type TicketTypeRepo interface {
	GetPublicTicketType(ctx context.Context, eventID, sessionID, ticketTypeID uuid.UUID) (domain.TicketType, error)
	GetPublicTicketTypes(ctx context.Context, eventID, sessionID uuid.UUID, limit, offset int) ([]domain.TicketType, error)

	CreateTicketType(
		ctx context.Context,
		ticketType domain.TicketType,
		eventID uuid.UUID,
		organizerID uuid.UUID,
		userID uuid.UUID,
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
		limit int,
		offset int,
		eventID uuid.UUID,
		sessionID uuid.UUID,
		organizerID uuid.UUID,
		userID uuid.UUID,
	) ([]domain.TicketType, error)
}

func NewTicketTypeService(
	ticketTypeRepo TicketTypeRepo,
) *TicketTypeService {
	return &TicketTypeService{
		ticketTypeRepo: ticketTypeRepo,
	}
}

type TicketTypesPage struct {
	TicketTypes []domain.TicketType
	Limit       int
	Offset      int
}
