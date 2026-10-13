package domain

import (
	"fmt"
	"github.com/google/uuid"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	"strings"
	"time"
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

func (t TicketType) Validate() error {
	if t.ID == uuid.Nil || t.SessionID == uuid.Nil || strings.TrimSpace(t.Name) == "" || t.Capacity <= 0 || t.Available < 0 || t.Held < 0 || t.Sold < 0 {
		return fmt.Errorf("%w: invalid ticket type", coreerrors.ErrInvalidArgument)
	}
	// Subtract rather than add counters, to avoid overflowing an int.
	if t.Available > t.Capacity || t.Held > t.Capacity-t.Available || t.Sold != t.Capacity-t.Available-t.Held {
		return fmt.Errorf("%w: inventory counters do not match capacity", coreerrors.ErrInvalidArgument)
	}
	return t.Price.Validate()
}
