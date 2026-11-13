package analytics_consumer_test

import (
	"encoding/json/v2"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	consumer "github.com/poponyas/TicketRush/internal/feature/analytics/consumer"
	repository "github.com/poponyas/TicketRush/internal/feature/analytics/repository"
	booking_events "github.com/poponyas/TicketRush/internal/feature/booking/events"
	"github.com/stretchr/testify/require"
)

func TestDecodeBookingEventLifecycle(t *testing.T) {
	for _, eventType := range []string{
		booking_events.BookingCreatedType,
		booking_events.BookingConfirmedType,
		booking_events.BookingCancelledType,
		booking_events.BookingExpiredType,
	} {
		t.Run(eventType, func(t *testing.T) {
			message, payload := validBookingMessage()
			message.Type = eventType
			data := encodeBookingMessage(t, message, payload)

			actual, err := consumer.DecodeBookingEvent(data)
			require.NoError(t, err)
			require.Equal(t, repository.BookingEvent{
				MessageID:     message.ID,
				BookingID:     payload.BookingID,
				EventID:       payload.EventID,
				SessionID:     payload.SessionID,
				TicketTypeID:  payload.TicketTypeID,
				EventType:     eventType,
				SchemaVersion: 1,
				Quantity:      2,
				OccurredAt:    message.CreatedAt.UTC(),
			}, actual)

			repeated, err := consumer.DecodeBookingEvent(data)
			require.NoError(t, err)
			require.Equal(t, actual, repeated)
		})
	}
}

func TestDecodeBookingEventRejectsInvalidData(t *testing.T) {
	overflowQuantity := uint64(math.MaxUint32) + 1
	cases := []struct {
		name   string
		mutate func(*consumer.Message, *booking_events.BookingLifecyclePayload)
	}{
		{"unknown type", func(m *consumer.Message, _ *booking_events.BookingLifecyclePayload) { m.Type = "booking.unknown" }},
		{"unsupported version", func(m *consumer.Message, _ *booking_events.BookingLifecyclePayload) { m.SchemaVersion = 2 }},
		{"missing message id", func(m *consumer.Message, _ *booking_events.BookingLifecyclePayload) { m.ID = uuid.Nil }},
		{"missing aggregate id", func(m *consumer.Message, _ *booking_events.BookingLifecyclePayload) { m.AggregateID = uuid.Nil }},
		{"missing timestamp", func(m *consumer.Message, _ *booking_events.BookingLifecyclePayload) { m.CreatedAt = time.Time{} }},
		{"missing payload id", func(_ *consumer.Message, p *booking_events.BookingLifecyclePayload) { p.EventID = uuid.Nil }},
		{"aggregate mismatch", func(m *consumer.Message, _ *booking_events.BookingLifecyclePayload) { m.AggregateID = uuid.New() }},
		{"zero quantity", func(_ *consumer.Message, p *booking_events.BookingLifecyclePayload) { p.Quantity = 0 }},
		{"negative quantity", func(_ *consumer.Message, p *booking_events.BookingLifecyclePayload) { p.Quantity = -1 }},
		{"quantity overflow", func(_ *consumer.Message, p *booking_events.BookingLifecyclePayload) {
			p.Quantity = int(overflowQuantity)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message, payload := validBookingMessage()
			tc.mutate(&message, &payload)
			actual, err := consumer.DecodeBookingEvent(encodeBookingMessage(t, message, payload))
			require.Error(t, err)
			require.Equal(t, repository.BookingEvent{}, actual)
		})
	}
	_, err := consumer.DecodeBookingEvent([]byte(`{"id":`))
	require.Error(t, err)

	message, _ := validBookingMessage()
	data, err := json.Marshal(message)
	require.NoError(t, err)
	_, err = consumer.DecodeBookingEvent(data)
	require.Error(t, err)
}

func validBookingMessage() (consumer.Message, booking_events.BookingLifecyclePayload) {
	payload := booking_events.BookingLifecyclePayload{
		BookingID: uuid.New(), EventID: uuid.New(), SessionID: uuid.New(),
		TicketTypeID: uuid.New(), Quantity: 2,
	}
	message := consumer.Message{
		ID: uuid.New(), AggregateID: payload.BookingID,
		Type: booking_events.BookingCreatedType, SchemaVersion: booking_events.BookingCreatedSchemaVersion,
		CreatedAt: time.Date(2026, time.November, 8, 19, 0, 0, 123456000, time.FixedZone("MSK", 3*60*60)),
	}
	return message, payload
}

func encodeBookingMessage(t *testing.T, message consumer.Message, payload booking_events.BookingLifecyclePayload) []byte {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	message.Payload = data
	data, err = json.Marshal(message)
	require.NoError(t, err)
	return data
}
