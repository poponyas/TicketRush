package events_repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *EventsRepository) PublishEvent(ctx context.Context, eventID uuid.UUID, organizerID uuid.UUID, userID uuid.UUID) (domain.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()

	tx, err := r.pool.Pool.Begin(ctx)
	if err != nil {
		return domain.Event{}, fmt.Errorf("begin event publication: %w", err)
	}
	defer func() {
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer rollbackCancel()
		_ = tx.Rollback(rollbackCtx)
	}()

	const lockQueryEvent = `
		SELECT e.status
		FROM events AS e
		JOIN organizer_memberships AS m ON m.organizer_id=e.organizer_id
		WHERE e.id=$1 AND e.organizer_id=$2
		  AND m.user_id=$3 AND m.role='organizer'
		FOR UPDATE OF e
		FOR SHARE OF m
	`
	var status domain.EventStatus
	err = tx.QueryRow(ctx, lockQueryEvent, eventID, organizerID, userID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Event{}, coreerrors.ErrNotFound
	}
	if err != nil {
		return domain.Event{}, fmt.Errorf("lock event for publication: %w", err)
	}
	if status != domain.EventDraft {
		return domain.Event{}, fmt.Errorf("%w: only draft events can be published", coreerrors.ErrConflict)
	}

	const sessionsQuery = `
		SELECT EXISTS (
			SELECT 1 FROM sessions
			WHERE event_id = $1
			  AND status = 'scheduled'
			  AND sales_end_at > clock_timestamp()
		)
	`
	var hasScheduledSession bool
	if err := tx.QueryRow(ctx, sessionsQuery, eventID).Scan(&hasScheduledSession); err != nil {
		return domain.Event{}, fmt.Errorf("check publishable sessions: %w", err)
	}
	if !hasScheduledSession {
		return domain.Event{}, fmt.Errorf("event needs a scheduled session with sales not yet ended: %w", coreerrors.ErrConflict)
	}

	const query = `
		UPDATE events
		SET status='published',
		    updated_at=GREATEST(clock_timestamp(), updated_at + INTERVAL '1 microsecond')
		WHERE id=$1 AND status='draft'
		RETURNING id, organizer_id, name, description, category,
		          poster_key, status, created_at, updated_at
	`
	rows, err := tx.Query(ctx, query, eventID)
	if err != nil {
		return domain.Event{}, fmt.Errorf("publish event: %w", err)
	}
	updated, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Event])
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Event{}, coreerrors.ErrConflict
	}
	if err != nil {
		return domain.Event{}, fmt.Errorf("collect published event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Event{}, fmt.Errorf("commit event publication: %w", err)
	}
	return updated, nil
}
