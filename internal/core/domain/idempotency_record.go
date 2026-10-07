package domain

import (
	"encoding/json"
	"github.com/google/uuid"
	"time"
)

// Unique scope: UserID + Operation + Key. Another payload with the same key conflicts.
type IdempotencyRecord struct {
	UserID      uuid.UUID
	Operation   string
	Key         string
	RequestHash string
	HTTPStatus  int
	Response    json.RawMessage
	CreatedAt   time.Time
	ExpiresAt   time.Time
}
