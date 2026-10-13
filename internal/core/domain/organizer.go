package domain

import (
	"github.com/google/uuid"
	"time"
)

type Organizer struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
}
