package session_repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *SessionRepository) UpdateSession(ctx context.Context, session, expected domain.Session, organizerID uuid.UUID, userID uuid.UUID) (domain.Session, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()

	tx, err := r.pool.Pool.Begin(ctx)
	if err != nil {
		return domain.Session{}, fmt.Errorf("create tx error: %w", err)
	}

	defer func() {
		rollbackContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = tx.Rollback(rollbackContext)
	}()

	const accessQuery = `
	SELECT e.status
	FROM events AS e
	JOIN organizer_memberships AS m
		ON m.organizer_id = e.organizer_id
	WHERE e.id = $1
	  AND e.organizer_id = $2
	  AND m.user_id = $3
	  AND m.role = 'organizer'
	FOR SHARE OF e, m
`

	var status domain.EventStatus

	if err := tx.QueryRow(
		ctx,
		accessQuery,
		session.EventID,
		organizerID,
		userID,
	).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, fmt.Errorf("not found: %w", core_errors.ErrNotFound)
		}

		return domain.Session{}, fmt.Errorf("check event access: %w", err)
	}

	if status != domain.EventDraft {
		return domain.Session{}, fmt.Errorf("event status must be draft: %w", core_errors.ErrConflict)
	}
	const query = `
	UPDATE sessions
	SET starts_at=$1, ends_at=$2
	WHERE id=$3 AND event_id=$4 AND status='draft'
	  AND starts_at=$5 AND ends_at IS NOT DISTINCT FROM $6::timestamptz
	  AND sales_start_at=$7 AND sales_end_at=$8
	RETURNING id,event_id,venue_id,starts_at,ends_at,sales_start_at,sales_end_at,status,created_at`
	rows, err := tx.Query(ctx, query, session.StartsAt, session.EndsAt, session.ID, session.EventID,
		expected.StartsAt, expected.EndsAt, expected.SalesStartAt, expected.SalesEndAt)
	if err != nil {
		return domain.Session{}, fmt.Errorf("update session: %w", err)
	}
	saved, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Session])
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sessions WHERE id=$1 AND event_id=$2)`, session.ID, session.EventID).Scan(&exists); err != nil {
			return domain.Session{}, fmt.Errorf("check session existence: %w", err)
		}
		if !exists {
			return domain.Session{}, core_errors.ErrNotFound
		}
		return domain.Session{}, fmt.Errorf("session changed or is not draft: %w", core_errors.ErrConflict)
	}
	if err != nil {
		return domain.Session{}, fmt.Errorf("read updated session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Session{}, fmt.Errorf("commit session: %w", err)
	}

	return saved, nil

}
