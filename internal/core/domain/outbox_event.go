package domain

import (
	"encoding/json"
	"github.com/google/uuid"
	"time"
)

type OutboxEvent struct {
	ID            uuid.UUID
	AggregateID   uuid.UUID
	Type          string
	SchemaVersion int
	Payload       json.RawMessage
	CreatedAt     time.Time
	PublishedAt   *time.Time
	Attempts      int
}
