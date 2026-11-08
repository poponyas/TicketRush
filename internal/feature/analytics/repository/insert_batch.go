package analytics_repository

import (
	"context"
	"fmt"
)

func (a *Analytics) InsertBatch(
	ctx context.Context,
	events []BookingEvent,
) error {
	if len(events) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, a.OpTimeout)
	defer cancel()

	const query = `
	INSERT INTO booking_events (
		message_id,
		booking_id,
		event_id,
		session_id,
		ticket_type_id,
		event_type,
		schema_version,
		quantity,
		occurred_at
	)
`

	batch, err := a.Conn.PrepareBatch(ctx, query)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	defer batch.Close()

	for _, event := range events {
		if err := batch.Append(
			event.MessageID,
			event.BookingID,
			event.EventID,
			event.SessionID,
			event.TicketTypeID,
			event.EventType,
			event.SchemaVersion,
			event.Quantity,
			event.OccurredAt,
		); err != nil {
			return fmt.Errorf(
				"append booking event %s: %w",
				event.MessageID,
				err,
			)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("send booking events batch: %w", err)
	}

	return nil
}
