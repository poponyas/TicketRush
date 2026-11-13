package domain

import (
	"time"

	"github.com/google/uuid"
)

type TicketStatus string

const (
	TicketActive  TicketStatus = "active"
	TicketUsed    TicketStatus = "used"
	TicketRevoked TicketStatus = "revoked"
)

// Ownership/session/tariff are read through booking_item and booking. QR images
// are generated on demand from a purpose-specific signed token, not stored in S3.
type Ticket struct {
	ID            uuid.UUID    `db:"id"`
	BookingID     uuid.UUID    `db:"booking_id"`
	BookingItemID uuid.UUID    `db:"booking_item_id"`
	Number        int          `db:"number"`
	UserID        uuid.UUID    `db:"user_id"`
	SessionID     uuid.UUID    `db:"session_id"`
	TicketTypeID  uuid.UUID    `db:"ticket_type_id"`
	Status        TicketStatus `db:"status"`
	CreatedAt     time.Time    `db:"created_at"`
	UsedAt        *time.Time   `db:"used_at"`
	CheckedInBy   *uuid.UUID   `db:"checked_in_by"`
}
