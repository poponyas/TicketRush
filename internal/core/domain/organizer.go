package domain

import (
	"github.com/google/uuid"
	"time"
)

type Organizer struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
}
