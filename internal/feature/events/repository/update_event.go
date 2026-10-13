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

func (r *EventsRepository) UpdateEvent(ctx context.Context, event domain.Event, userID uuid.UUID) (domain.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()

	tx, err := r.pool.Pool.Begin(ctx)
	if err != nil {
		return domain.Event{}, fmt.Errorf("begin event update: %w", err)
	}
	defer func() {
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer rollbackCancel()
		_ = tx.Rollback(rollbackCtx)
	}()

	const lockQuery = `
		SELECT e.status, e.updated_at
		FROM events AS e
		JOIN organizer_memberships AS m ON m.organizer_id=e.organizer_id
		WHERE e.id=$1 AND e.organizer_id=$2
		  AND m.user_id=$3 AND m.role='organizer'
		FOR UPDATE OF e
		FOR SHARE OF m
	`
	var status domain.EventStatus
	var updatedAt time.Time
	err = tx.QueryRow(ctx, lockQuery, event.ID, event.OrganizerID, userID).Scan(&status, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Event{}, coreerrors.ErrNotFound
	}
	if err != nil {
		return domain.Event{}, fmt.Errorf("lock event for update: %w", err)
	}
	if status != domain.EventDraft {
		return domain.Event{}, fmt.Errorf("%w: only draft events can be updated", coreerrors.ErrConflict)
	}

	if !updatedAt.Equal(event.UpdatedAt) {
		return domain.Event{}, fmt.Errorf("%w: event changed; fetch it again and retry", coreerrors.ErrConflict)
	}

	const query = `
		UPDATE events
		SET name=$3, description=$4, category=$5,
		    updated_at=GREATEST(clock_timestamp(), updated_at + INTERVAL '1 microsecond')
		WHERE id=$1 AND organizer_id=$2 AND status='draft' AND updated_at=$6
		RETURNING id, organizer_id, name, description, category,
		          poster_key, status, created_at, updated_at
	`
	rows, err := tx.Query(ctx, query, event.ID, event.OrganizerID,
		event.Title, event.Description, event.Category, event.UpdatedAt)
	if err != nil {
		return domain.Event{}, fmt.Errorf("query event update: %w", err)
	}
	updated, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Event])
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Event{}, coreerrors.ErrConflict
	}
	if err != nil {
		return domain.Event{}, fmt.Errorf("collect updated event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Event{}, fmt.Errorf("commit event update: %w", err)
	}
	return updated, nil
}
