package outbox_publisher

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	"github.com/segmentio/kafka-go"
)

type eventMessage struct {
	ID            uuid.UUID      `json:"id"`
	AggregateID   uuid.UUID      `json:"aggregate_id"`
	Type          string         `json:"type"`
	SchemaVersion int            `json:"schema_version"`
	CreatedAt     time.Time      `json:"created_at"`
	Payload       jsontext.Value `json:"payload"`
}

func (k *KafkaPublisher) Publish(
	ctx context.Context,
	event domain.OutboxEvent,
) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	message := eventMessage{
		ID:            event.ID,
		AggregateID:   event.AggregateID,
		Type:          event.Type,
		SchemaVersion: event.SchemaVersion,
		CreatedAt:     event.CreatedAt,
		Payload:       event.Payload,
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal outbox event %s: %w", event.ID, err)
	}

	if err := k.kafkaWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.AggregateID.String()),
		Value: data,
	}); err != nil {
		return fmt.Errorf("publish outbox event %s: %w", event.ID, err)
	}

	return nil
}
