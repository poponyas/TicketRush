package outbox_repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
)

func (r *OutboxRepository) GetPendingEvents(
	ctx context.Context,
	limit int,
) ([]domain.OutboxEvent, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive")
	}
	ctx, cancel := context.WithTimeout(ctx, r.Pool.OpTimeout)
	defer cancel()

	const query = `
	SELECT id, aggregate_id, type, schema_version, payload,
		created_at, published_at, attempts
	FROM outbox_events
	WHERE published_at IS NULL
	ORDER BY created_at, id
	LIMIT $1;
	`

	rows, err := r.Pool.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}

	outboxEvents, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.OutboxEvent])
	if err != nil {
		return nil, fmt.Errorf("collect rows: %w", err)
	}

	return outboxEvents, nil
}
