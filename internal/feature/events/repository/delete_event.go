package events_repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *EventsRepository) DeleteEvent(ctx context.Context, eventID, organizerID, userID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	tx, err := r.pool.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin event deletion: %w", err)
	}
	defer func() {
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer rollbackCancel()
		_ = tx.Rollback(rollbackCtx)
	}()

	const lockQuery = `
		SELECT e.status
		FROM events AS e
		JOIN organizer_memberships AS m ON m.organizer_id=e.organizer_id
		WHERE e.id=$1 AND e.organizer_id=$2
		  AND m.user_id=$3 AND m.role='organizer'
		FOR UPDATE OF e
		FOR SHARE OF m
	`
	var status domain.EventStatus
	err = tx.QueryRow(ctx, lockQuery, eventID, organizerID, userID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return coreerrors.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock event for deletion: %w", err)
	}
	if status != domain.EventDraft {
		return fmt.Errorf("%w: only draft events can be deleted", coreerrors.ErrConflict)
	}

	tag, err := tx.Exec(ctx, `DELETE FROM events WHERE id=$1 AND organizer_id=$2 AND status='draft'`, eventID, organizerID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return fmt.Errorf("%w: event has dependent records", coreerrors.ErrConflict)
		}
		return fmt.Errorf("delete event row: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return coreerrors.ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit event deletion: %w", err)
	}
	return nil
}
