package booking_repository

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	booking_events "github.com/poponyas/TicketRush/internal/feature/booking/events"
)

func insertBookingOutboxEvent(
	ctx context.Context,
	tx pgx.Tx,
	payload booking_events.BookingLifecyclePayload,
	eventType string,
	schemaVersion int,
	occurredAt time.Time,
) error {
	if payload.Quantity <= 0 || uint64(payload.Quantity) > math.MaxUint32 {
		return fmt.Errorf("booking event quantity must fit a positive UInt32: %w", coreerrors.ErrConflict)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s payload: %w", eventType, err)
	}
	const query = `INSERT INTO outbox_events(id,aggregate_id,type,schema_version,payload,created_at)
  VALUES($1,$2,$3,$4,$5,$6)`
	if _, err := tx.Exec(ctx, query, uuid.New(), payload.BookingID, eventType, schemaVersion, string(data), occurredAt); err != nil {
		return fmt.Errorf("insert %s outbox event: %w", eventType, err)
	}
	return nil
}

// Call after the booking rows are locked and their statuses have been changed.
// The v1 event contains one tariff, so group repeated positions of that tariff.
func insertBookingLifecycleEvents(
	ctx context.Context,
	tx pgx.Tx,
	bookingIDs []uuid.UUID,
	eventType string,
	schemaVersion int,
) error {
	const query = `SELECT b.id,s.event_id,b.session_id,i.ticket_type_id,SUM(i.quantity)::bigint
  FROM booking b
  JOIN sessions s ON s.id=b.session_id
  JOIN booking_item i ON i.booking_id=b.id
  WHERE b.id=ANY($1::uuid[])
  GROUP BY b.id,s.event_id,b.session_id,i.ticket_type_id
  ORDER BY b.id,i.ticket_type_id`
	rows, err := tx.Query(ctx, query, bookingIDs)
	if err != nil {
		return fmt.Errorf("query %s booking positions: %w", eventType, err)
	}
	payloads, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (booking_events.BookingLifecyclePayload, error) {
		var payload booking_events.BookingLifecyclePayload
		err := row.Scan(&payload.BookingID, &payload.EventID, &payload.SessionID, &payload.TicketTypeID, &payload.Quantity)
		return payload, err
	})
	if err != nil {
		return fmt.Errorf("read %s booking positions: %w", eventType, err)
	}

	seen := make(map[uuid.UUID]struct{}, len(bookingIDs))
	for _, payload := range payloads {
		seen[payload.BookingID] = struct{}{}
	}
	if len(seen) != len(bookingIDs) {
		return fmt.Errorf("some bookings have no positions for %s: %w", eventType, coreerrors.ErrConflict)
	}

	// This is the transition's processing time, not the booking's creation time.
	occurredAt := time.Now().UTC()
	for _, payload := range payloads {
		if err := insertBookingOutboxEvent(ctx, tx, payload, eventType, schemaVersion, occurredAt); err != nil {
			return err
		}
	}
	return nil
}
