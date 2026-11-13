package events_repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

// Ownership and current membership are checked in the modifying statement too.
func (r *EventsRepository) SetPoster(ctx context.Context, event, org, user uuid.UUID, key string) error {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	var id uuid.UUID
	err := r.pool.Pool.QueryRow(ctx, `UPDATE events e SET poster_key=$4,
		updated_at=GREATEST(clock_timestamp(),e.updated_at+interval '1 microsecond')
		WHERE e.id=$1 AND e.organizer_id=$2 AND e.status IN ('draft','published') AND EXISTS(
		 SELECT 1 FROM organizer_memberships m WHERE m.organizer_id=e.organizer_id AND m.user_id=$3 AND m.role='organizer')
		RETURNING e.id`, event, org, user, key).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return coreerrors.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("set event poster: %w", err)
	}
	return nil
}
