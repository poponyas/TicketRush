package domain

import (
	"github.com/google/uuid"
	"time"
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
	ID           uuid.UUID
	OrderID      uuid.UUID
	UserID       uuid.UUID
	SessionID    uuid.UUID
	TicketTypeID uuid.UUID
	TokenHash    string `json:"-"`
	QRObjectKey  string `json:"-"`
	Status       TicketStatus
	CreatedAt    time.Time
	UsedAt       *time.Time
	CheckedInBy  *uuid.UUID
}
