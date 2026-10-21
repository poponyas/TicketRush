package ticket_type_transport

import (
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
)

type TicketTypeRequest struct {
	Name     string       `json:"name"`
	Price    domain.Money `json:"price"`
	Capacity int          `json:"capacity"`
}

type TicketTypeResponse struct {
	ID        uuid.UUID    `json:"id"`
	SessionID uuid.UUID    `json:"session_id"`
	Name      string       `json:"name"`
	Price     domain.Money `json:"price"`
	Capacity  int          `json:"capacity"`
	Available int          `json:"available"`
	CreatedAt time.Time    `json:"created_at"`
}

type TicketTypesResponse []TicketTypeResponse

func domainToTicketTypeResponse(
	dmn domain.TicketType,
) TicketTypeResponse {
	return TicketTypeResponse{
		ID:        dmn.ID,
		SessionID: dmn.SessionID,
		Name:      dmn.Name,
		Price:     dmn.Price,
		Capacity:  dmn.Capacity,
		Available: dmn.Available,
		CreatedAt: dmn.CreatedAt,
	}
}

func domainToTicketTypesResponse(
	dmn []domain.TicketType,
) TicketTypesResponse {
	ticketTypes := make([]TicketTypeResponse, len(dmn))
	for i, tt := range dmn {
		ticketTypes[i] = domainToTicketTypeResponse(tt)
	}
	return ticketTypes
}
