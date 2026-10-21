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

func (r *SessionRepository) GetPublicSession(ctx context.Context, eventID, sessionID uuid.UUID) (domain.Session, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	const query = `SELECT s.id,s.event_id,s.venue_id,s.starts_at,s.ends_at,s.sales_start_at,s.sales_end_at,s.status,s.created_at
 FROM sessions s JOIN events e ON e.id=s.event_id
 WHERE e.id=$1 AND s.id=$2 AND e.status='published' AND s.status='scheduled' AND s.starts_at > CURRENT_TIMESTAMP`
	rows, err := r.pool.Pool.Query(ctx, query, eventID, sessionID)
	if err != nil {
		return domain.Session{}, fmt.Errorf("query public session: %w", err)
	}
	session, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Session])
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, coreerrors.ErrNotFound
	}
	if err != nil {
		return domain.Session{}, fmt.Errorf("collect public session: %w", err)
	}
	return session, nil
}

func (r *SessionRepository) GetPublicSessions(ctx context.Context, eventID uuid.UUID, limit, offset int) ([]domain.Session, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	var visible bool
	if err := r.pool.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM events WHERE id=$1 AND status='published')`, eventID).Scan(&visible); err != nil {
		return nil, fmt.Errorf("check public event: %w", err)
	}
	if !visible {
		return nil, coreerrors.ErrNotFound
	}
	const query = `SELECT s.id,s.event_id,s.venue_id,s.starts_at,s.ends_at,s.sales_start_at,s.sales_end_at,s.status,s.created_at
 FROM sessions s JOIN events e ON e.id=s.event_id
 WHERE e.id=$1 AND e.status='published' AND s.status='scheduled' AND s.starts_at > CURRENT_TIMESTAMP
 ORDER BY s.starts_at ASC,s.id ASC LIMIT $2 OFFSET $3`
	rows, err := r.pool.Pool.Query(ctx, query, eventID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query public sessions: %w", err)
	}
	sessions, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.Session])
	if err != nil {
		return nil, fmt.Errorf("collect public sessions: %w", err)
	}
	return sessions, nil
}
