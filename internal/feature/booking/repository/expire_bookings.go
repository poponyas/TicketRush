package booking_repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

// ExpireBookings commits one bounded batch and returns the number expired.
func (r *BookingRepository) ExpireBookings(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 1000 {
		return 0, fmt.Errorf("expiration batch size must be between 1 and 1000: %w", coreerrors.ErrInvalidArgument)
	}
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	tx, err := r.pool.Pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin booking expiration: %w", err)
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	}()

	// SKIP LOCKED gives concurrent workers different batches and skips a booking
	// currently handled by cancellation. The stable timestamp supports index scans.
	const query = `SELECT id FROM booking
  WHERE status='held' AND expires_at <= statement_timestamp()
  ORDER BY expires_at ASC,id ASC LIMIT $1
  FOR UPDATE SKIP LOCKED`
	rows, err := tx.Query(ctx, query, limit)
	if err != nil {
		return 0, fmt.Errorf("select expired bookings: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return 0, fmt.Errorf("collect expired bookings: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}
	if err := releaseHeldTickets(ctx, tx, ids); err != nil {
		return 0, err
	}
	const updateQuery = `UPDATE booking SET status='expired'
  WHERE id=ANY($1::uuid[]) AND status='held' AND expires_at <= clock_timestamp()`
	tag, err := tx.Exec(ctx, updateQuery, ids)
	if err != nil {
		return 0, fmt.Errorf("mark expired bookings: %w", err)
	}
	if tag.RowsAffected() != int64(len(ids)) {
		return 0, fmt.Errorf("expiration batch changed unexpectedly: %w", coreerrors.ErrConflict)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit booking expiration: %w", err)
	}
	return len(ids), nil
}
