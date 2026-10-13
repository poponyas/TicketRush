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

// TokenHash is the hash of a random QR token. The private QR image is stored in S3.
// Neither the token hash nor the object key belongs in a public response.
type Ticket struct {
	ID           uuid.UUID    `db:"id"`
	OrderID      uuid.UUID    `db:"order_id"`
	UserID       uuid.UUID    `db:"user_id"`
	SessionID    uuid.UUID    `db:"session_id"`
	TicketTypeID uuid.UUID    `db:"ticket_type_id"`
	TokenHash    string       `json:"-" db:"token_hash"`
	QRObjectKey  string       `json:"-" db:"qr_object_key"`
	Status       TicketStatus `db:"status"`
	CreatedAt    time.Time    `db:"created_at"`
	UsedAt       *time.Time   `db:"used_at"`
	CheckedInBy  *uuid.UUID   `db:"checked_in_by"`
}
