package domain

import (
	"encoding/json/jsontext"
	"github.com/google/uuid"
	"time"
)

type OutboxEvent struct {
	ID            uuid.UUID      `db:"id"`
	AggregateID   uuid.UUID      `db:"aggregate_id"`
	Type          string         `db:"type"`
	SchemaVersion int            `db:"schema_version"`
	Payload       jsontext.Value `db:"payload"`
	CreatedAt     time.Time      `db:"created_at"`
	PublishedAt   *time.Time     `db:"published_at"`
	Attempts      int            `db:"attempts"`
}
