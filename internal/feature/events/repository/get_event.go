package events_repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *EventsRepository) GetEvent(
	ctx context.Context,
	eventID uuid.UUID,
	organizerID uuid.UUID,
	userID uuid.UUID,
) (domain.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()

	const query = `
		SELECT
			e.id,
			e.organizer_id,
			e.name,
			e.description,
			e.category,
			e.poster_key,
			e.status,
			e.created_at,
			e.updated_at
		FROM events AS e
		WHERE e.id = $1
		  AND e.organizer_id = $2
		  AND EXISTS (
			  SELECT 1
			  FROM organizer_memberships AS m
			  WHERE m.organizer_id = e.organizer_id
			    AND m.user_id = $3
			    AND m.role = 'organizer'
		  )
	`

	rows, err := r.pool.Pool.Query(
		ctx, query, eventID, organizerID, userID,
	)
	if err != nil {
		return domain.Event{}, fmt.Errorf("query event: %w", err)
	}

	event, err := pgx.CollectExactlyOneRow(
		rows,
		pgx.RowToStructByName[domain.Event],
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Event{}, core_errors.ErrNotFound
	}
	if err != nil {
		return domain.Event{}, fmt.Errorf("collect event: %w", err)
	}

	return event, nil
}
