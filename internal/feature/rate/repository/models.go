package ticket_type_repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	"github.com/shopspring/decimal"
)

// ticketTypeModel mirrors a PostgreSQL row: price and currency are separate columns.
type ticketTypeModel struct {
	ID        uuid.UUID       `db:"id"`
	SessionID uuid.UUID       `db:"session_id"`
	Name      string          `db:"name"`
	Price     decimal.Decimal `db:"price"`
	Currency  string          `db:"currency"`
	Capacity  int             `db:"capacity"`
	Available int             `db:"available"`
	Held      int             `db:"held"`
	Sold      int             `db:"sold"`
	CreatedAt time.Time       `db:"created_at"`
}

func toDomain(m ticketTypeModel) domain.TicketType {
	return domain.TicketType{
		ID: m.ID, SessionID: m.SessionID, Name: m.Name,
		Price:    domain.Money{Amount: m.Price, Currency: m.Currency},
		Capacity: m.Capacity, Available: m.Available, Held: m.Held, Sold: m.Sold, CreatedAt: m.CreatedAt,
	}
}

func modelsToDomain(
	dmn []ticketTypeModel,
) []domain.TicketType {
	ticketTypes := make([]domain.TicketType, len(dmn))
	for i, tt := range dmn {
		ticketTypes[i] = toDomain(tt)
	}
	return ticketTypes
}
