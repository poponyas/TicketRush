package booking_repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	"github.com/shopspring/decimal"
)

// Item fields are nullable because a LEFT JOIN also returns bookings without items.
type bookingDetailsRow struct {
	BookingID    uuid.UUID            `db:"booking_id"`
	UserID       uuid.UUID            `db:"user_id"`
	SessionID    uuid.UUID            `db:"session_id"`
	Status       domain.BookingStatus `db:"status"`
	ExpiresAt    time.Time            `db:"expires_at"`
	CreatedAt    time.Time            `db:"created_at"`
	ItemID       *uuid.UUID           `db:"item_id"`
	TicketTypeID *uuid.UUID           `db:"ticket_type_id"`
	Quantity     *int                 `db:"quantity"`
	UnitPrice    *decimal.Decimal     `db:"unit_price"`
	Currency     *string              `db:"currency"`
}
