package events_repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *EventsRepository) GetEvents(ctx context.Context,
	limit int,
	offset int,
	organizerID uuid.UUID,
	userID uuid.UUID,
) ([]domain.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()

	const membershipQuery = `SELECT EXISTS (
		SELECT 1 FROM organizer_memberships
		WHERE organizer_id=$1 AND user_id=$2 AND role='organizer'
	)`
	var isMember bool
	if err := r.pool.Pool.QueryRow(ctx, membershipQuery, organizerID, userID).Scan(&isMember); err != nil {
		return nil, fmt.Errorf("check organizer membership: %w", err)
	}
	if !isMember {
		return nil, core_errors.ErrForbidden
	}

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
	JOIN organizer_memberships AS m
		ON m.organizer_id = e.organizer_id
	WHERE e.organizer_id = $3
	AND m.user_id = $4
	AND m.role = 'organizer'
	ORDER BY e.created_at DESC, e.id DESC
	LIMIT $1
	OFFSET $2
	`

	rows, err := r.pool.Pool.Query(ctx, query, limit, offset, organizerID, userID)
	if err != nil {
		return nil, fmt.Errorf("query event: %w", err)
	}

	events, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.Event])
	if err != nil {
		return nil, fmt.Errorf("collect events: %w", err)
	}

	return events, nil
}
