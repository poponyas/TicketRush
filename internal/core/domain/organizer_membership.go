package domain

import (
	"github.com/google/uuid"
	"time"
)

// One user may belong to several organizers and also buy tickets.
type OrganizerMembership struct {
	OrganizerID uuid.UUID
	UserID      uuid.UUID
	Role        Role
	CreatedAt   time.Time
}
