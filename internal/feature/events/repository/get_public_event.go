package events_repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *EventsRepository) GetPublicEvent(ctx context.Context, eventID uuid.UUID) (domain.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	const query = `
  SELECT id, organizer_id, name, description, category,
         poster_key, status, created_at, updated_at
  FROM events
  WHERE id = $1 AND status = 'published'
 `
	rows, err := r.pool.Pool.Query(ctx, query, eventID)
	if err != nil {
		return domain.Event{}, fmt.Errorf("query public event: %w", err)
	}
	event, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Event])
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Event{}, coreerrors.ErrNotFound
	}
	if err != nil {
		return domain.Event{}, fmt.Errorf("collect public event: %w", err)
	}
	return event, nil
}
