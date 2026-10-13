package domain

import (
	"encoding/json"
	"github.com/google/uuid"
	"time"
)

// Unique scope: UserID + Operation + Key. Another payload with the same key conflicts.
type IdempotencyRecord struct {
	UserID      uuid.UUID       `db:"user_id"`
	Operation   string          `db:"operation"`
	Key         string          `db:"key"`
	RequestHash string          `db:"request_hash"`
	HTTPStatus  int             `db:"http_status"`
	Response    json.RawMessage `db:"response"`
	CreatedAt   time.Time       `db:"created_at"`
	ExpiresAt   time.Time       `db:"expires_at"`
}
