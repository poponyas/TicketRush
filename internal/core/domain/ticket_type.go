package domain

import (
	"fmt"
	"github.com/google/uuid"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	"strings"
	"time"
)

type TicketType struct {
	ID        uuid.UUID
	SessionID uuid.UUID
	Name      string
	Price     Money
	Capacity  int
	Available int
	Held      int
	Sold      int
	CreatedAt time.Time
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
