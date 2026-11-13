package analytics_consumer

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	analytics_repository "github.com/poponyas/TicketRush/internal/feature/analytics/repository"
	booking_events "github.com/poponyas/TicketRush/internal/feature/booking/events"
)

type Message struct {
	ID            uuid.UUID      `json:"id"`
	AggregateID   uuid.UUID      `json:"aggregate_id"`
	Type          string         `json:"type"`
	SchemaVersion int            `json:"schema_version"`
	CreatedAt     time.Time      `json:"created_at"`
	Payload       jsontext.Value `json:"payload"`
}

func DecodeBookingEvent(
	data []byte,
) (analytics_repository.BookingEvent, error) {
	var message Message
	if err := json.Unmarshal(data, &message, json.RejectUnknownMembers(true)); err != nil {
		return analytics_repository.BookingEvent{}, fmt.Errorf("unmarshal json: %w", err)
	}

	var expectedVersion int
	switch message.Type {
	case booking_events.BookingConfirmedType:
		expectedVersion = booking_events.BookingConfirmedSchemaVersion
	case booking_events.BookingCreatedType:
		expectedVersion = booking_events.BookingCreatedSchemaVersion
	case booking_events.BookingCancelledType:
		expectedVersion = booking_events.BookingCancelledSchemaVersion
	case booking_events.BookingExpiredType:
		expectedVersion = booking_events.BookingExpiredSchemaVersion
	default:
		return analytics_repository.BookingEvent{}, fmt.Errorf("unsupported booking event type %q", message.Type)
	}

	if message.SchemaVersion != expectedVersion {
		return analytics_repository.BookingEvent{}, fmt.Errorf(
			"unsupported schema version %d for %q: want %d",
			message.SchemaVersion, message.Type, expectedVersion,
		)
	}

	if message.ID == uuid.Nil || message.AggregateID == uuid.Nil {
		return analytics_repository.BookingEvent{}, fmt.Errorf("message id and aggregate_id must not be nil")
	}
	if message.CreatedAt.IsZero() {
		return analytics_repository.BookingEvent{}, fmt.Errorf("message created_at must not be zero")
	}

	var payload booking_events.BookingLifecyclePayload
	if err := json.Unmarshal(message.Payload, &payload, json.RejectUnknownMembers(true)); err != nil {
		return analytics_repository.BookingEvent{}, fmt.Errorf("unmarshal booking event payload: %w", err)
	}

	if payload.BookingID == uuid.Nil || payload.EventID == uuid.Nil ||
		payload.SessionID == uuid.Nil || payload.TicketTypeID == uuid.Nil {
		return analytics_repository.BookingEvent{}, fmt.Errorf("booking event payload ids must not be nil")
	}
	if message.AggregateID != payload.BookingID {
		return analytics_repository.BookingEvent{}, fmt.Errorf("aggregate_id must match payload booking_id")
	}
	if payload.Quantity <= 0 || uint64(payload.Quantity) > math.MaxUint32 {
		return analytics_repository.BookingEvent{}, fmt.Errorf("quantity must be between 1 and %d", uint64(math.MaxUint32))
	}

	return analytics_repository.BookingEvent{
		MessageID:     message.ID,
		BookingID:     payload.BookingID,
		EventID:       payload.EventID,
		SessionID:     payload.SessionID,
		TicketTypeID:  payload.TicketTypeID,
		EventType:     message.Type,
		SchemaVersion: uint32(message.SchemaVersion),
		Quantity:      uint32(payload.Quantity),
		OccurredAt:    message.CreatedAt.UTC(),
	}, nil
}
