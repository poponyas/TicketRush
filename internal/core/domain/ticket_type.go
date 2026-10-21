package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

type TicketType struct {
	ID        uuid.UUID `db:"id"`
	SessionID uuid.UUID `db:"session_id"`
	Name      string    `db:"name"`
	// Price uses two SQL columns (price, currency); map them explicitly in the repository.
	Price     Money     `db:"-"`
	Capacity  int       `db:"capacity"`
	Available int       `db:"available"`
	Held      int       `db:"held"`
	Sold      int       `db:"sold"`
	CreatedAt time.Time `db:"created_at"`
}

func NewTicketType(
	sessionID uuid.UUID,
	name string,
	price Money,
	capacity int,
) TicketType {
	ticketID := uuid.New()
	return TicketType{
		ID:        ticketID,
		SessionID: sessionID,
		Name:      strings.TrimSpace(name),
		Price:     price,
		Capacity:  capacity,
		Available: capacity,
		Held:      0,
		Sold:      0,
	}
}

func (t TicketType) Validate() error {
	if t.ID == uuid.Nil || t.SessionID == uuid.Nil || strings.TrimSpace(t.Name) == "" || t.Capacity <= 0 || t.Capacity > 1<<31-1 || t.Available < 0 || t.Held < 0 || t.Sold < 0 {
		return fmt.Errorf("%w: invalid ticket type", coreerrors.ErrInvalidArgument)
	}

	if utf8.RuneCountInString(t.Name) > 100 || strings.ContainsRune(t.Name, '\x00') {
		return fmt.Errorf("invalid name: %w", coreerrors.ErrInvalidArgument)
	}

	if t.Available > t.Capacity || t.Held > t.Capacity-t.Available || t.Sold != t.Capacity-t.Available-t.Held {
		return fmt.Errorf("%w: inventory counters do not match capacity", coreerrors.ErrInvalidArgument)
	}
	return t.Price.Validate()
}
