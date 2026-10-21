package events_repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
)

func (r *EventsRepository) GetPublicEvents(ctx context.Context, limit, offset int) ([]domain.Event, error) {
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
	WHERE e.status = 'published'
	ORDER BY e.created_at DESC, e.id DESC
	LIMIT $1
	OFFSET $2
	`

	rows, err := r.pool.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query event: %w", err)
	}

	events, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.Event])
	if err != nil {
		return nil, fmt.Errorf("collect events: %w", err)
	}

	return events, nil
}
