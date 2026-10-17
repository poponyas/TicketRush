package session_repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *SessionRepository) GetSession(ctx context.Context, eventID, sessionID, organizerID, userID uuid.UUID) (domain.Session, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	const query = `SELECT s.id,s.event_id,s.venue_id,s.starts_at,s.ends_at,s.sales_start_at,s.sales_end_at,s.status,s.created_at
		FROM sessions AS s
		JOIN events AS e ON e.id=s.event_id
		JOIN organizer_memberships AS m ON m.organizer_id=e.organizer_id
		WHERE s.event_id=$1 AND s.id=$2 AND e.organizer_id=$3
		  AND m.user_id=$4 AND m.role='organizer'`
	rows, err := r.pool.Pool.Query(ctx, query, eventID, sessionID, organizerID, userID)
	if err != nil {
		return domain.Session{}, fmt.Errorf("query session: %w", err)
	}
	session, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Session])
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, coreerrors.ErrNotFound
	}
	if err != nil {
		return domain.Session{}, fmt.Errorf("read session: %w", err)
	}
	return session, nil
}
