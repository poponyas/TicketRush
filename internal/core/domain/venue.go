package domain

import (
	"github.com/google/uuid"
	"time"
)

type Venue struct {
	ID        uuid.UUID
	Name      string
	City      string
	Address   string
	Timezone  string
	CreatedAt time.Time
}
