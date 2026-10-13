package domain

import (
	"github.com/google/uuid"
	"time"
)

// One user may belong to several organizers and also buy tickets.
type OrganizerMembership struct {
	OrganizerID uuid.UUID `db:"organizer_id"`
	UserID      uuid.UUID `db:"user_id"`
	Role        Role      `db:"role"`
	CreatedAt   time.Time `db:"created_at"`
}
