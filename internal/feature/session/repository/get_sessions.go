package session_repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *SessionRepository) GetSessions(ctx context.Context, eventID, organizerID, userID uuid.UUID, limit, offset int) ([]domain.Session, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	// An empty accessible event is 200 []; a missing or inaccessible parent is 404.
	const accessQuery = `SELECT EXISTS(SELECT 1 FROM events AS e
		JOIN organizer_memberships AS m ON m.organizer_id=e.organizer_id
		WHERE e.id=$1 AND e.organizer_id=$2 AND m.user_id=$3 AND m.role='organizer')`
	var allowed bool
	if err := r.pool.Pool.QueryRow(ctx, accessQuery, eventID, organizerID, userID).Scan(&allowed); err != nil {
		return nil, fmt.Errorf("check event access: %w", err)
	}
	if !allowed {
		return nil, coreerrors.ErrNotFound
	}
	const query = `SELECT s.id,s.event_id,s.venue_id,s.starts_at,s.ends_at,s.sales_start_at,s.sales_end_at,s.status,s.created_at
		FROM sessions AS s
		JOIN events AS e ON e.id=s.event_id
		JOIN organizer_memberships AS m ON m.organizer_id=e.organizer_id
		WHERE s.event_id=$1 AND e.organizer_id=$2 AND m.user_id=$3 AND m.role='organizer'
		ORDER BY s.starts_at ASC,s.id ASC LIMIT $4 OFFSET $5`
	rows, err := r.pool.Pool.Query(ctx, query, eventID, organizerID, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	sessions, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.Session])
	if err != nil {
		return nil, fmt.Errorf("read sessions: %w", err)
	}
	return sessions, nil
}
