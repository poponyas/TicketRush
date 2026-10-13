package domain

import (
	"github.com/google/uuid"
	"time"
)

type Venue struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	City      string    `db:"city"`
	Address   string    `db:"address"`
	Timezone  string    `db:"timezone"`
	CreatedAt time.Time `db:"created_at"`
}
