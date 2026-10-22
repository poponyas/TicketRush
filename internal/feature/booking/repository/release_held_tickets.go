package booking_repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

// Call only after locking every booking whose inventory is being released.
// Group positions across the batch and update tariffs in a global order so two
// workers and manual cancellation acquire tariff locks consistently.
func releaseHeldTickets(ctx context.Context, tx pgx.Tx, bookingIDs []uuid.UUID) error {
	const query = `SELECT i.ticket_type_id,b.session_id,SUM(i.quantity)::bigint AS quantity
  FROM booking_item i JOIN booking b ON b.id=i.booking_id
  WHERE b.id=ANY($1::uuid[])
  GROUP BY i.ticket_type_id,b.session_id
  ORDER BY i.ticket_type_id ASC,b.session_id ASC`
	type releaseItem struct {
		TicketTypeID uuid.UUID `db:"ticket_type_id"`
		SessionID    uuid.UUID `db:"session_id"`
		Quantity     int64     `db:"quantity"`
	}
	rows, err := tx.Query(ctx, query, bookingIDs)
	if err != nil {
		return fmt.Errorf("query held booking items: %w", err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[releaseItem])
	if err != nil {
		return fmt.Errorf("read held booking items: %w", err)
	}
	for _, item := range items {
		const updateQuery = `UPDATE ticket_types
   SET available=available+$1, held=held-$1
   WHERE id=$2 AND session_id=$3 AND held >= $1`
		tag, err := tx.Exec(ctx, updateQuery, item.Quantity, item.TicketTypeID, item.SessionID)
		if err != nil {
			return fmt.Errorf("release held tickets: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("held inventory does not match booking: %w", coreerrors.ErrConflict)
		}
	}
	return nil
}
