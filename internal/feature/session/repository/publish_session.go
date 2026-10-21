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

func (r *SessionRepository) PublishSession(
	ctx context.Context,
	eventID, sessionID, organizerID, userID uuid.UUID,
) (domain.Session, error) {
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
		eventID,
		organizerID,
		userID,
	).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, fmt.Errorf("not found: %w", core_errors.ErrNotFound)
		}

		return domain.Session{}, fmt.Errorf("check event access: %w", err)
	}

	if status != domain.EventDraft && status != domain.EventPublished {
		return domain.Session{}, fmt.Errorf("event must be draft or published: %w", core_errors.ErrConflict)
	}

	const sessionQuery = `
	SELECT id, event_id, venue_id, starts_at, ends_at,
	       sales_start_at, sales_end_at, status, created_at
	FROM sessions
	WHERE id = $1 AND event_id = $2
	FOR UPDATE
	`
	rows, err := tx.Query(ctx, sessionQuery, sessionID, eventID)
	if err != nil {
		return domain.Session{}, fmt.Errorf("lock session for publication: %w", err)
	}
	session, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Session])
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, fmt.Errorf("session not found: %w", core_errors.ErrNotFound)
	}
	if err != nil {
		return domain.Session{}, fmt.Errorf("read session for publication: %w", err)
	}
	if session.Status != domain.SessionDraft {
		return domain.Session{}, fmt.Errorf("session must be draft: %w", core_errors.ErrConflict)
	}
	if err := session.Validate(); err != nil {
		return domain.Session{}, fmt.Errorf("invalid session schedule: %v: %w", err, core_errors.ErrConflict)
	}

	const tariffsQuery = `SELECT EXISTS(SELECT 1 FROM ticket_types WHERE session_id = $1)`
	var hasTariffs bool
	if err := tx.QueryRow(ctx, tariffsQuery, sessionID).Scan(&hasTariffs); err != nil {
		return domain.Session{}, fmt.Errorf("check session tariffs: %w", err)
	}
	if !hasTariffs {
		return domain.Session{}, fmt.Errorf("session must have at least one tariff: %w", core_errors.ErrConflict)
	}

	now := time.Now().UTC()
	if !session.StartsAt.After(now) || !session.SalesEndAt.After(now) {
		return domain.Session{}, fmt.Errorf("session start and sales end must be in the future: %w", core_errors.ErrConflict)
	}

	const publishQuery = `
	UPDATE sessions
	SET status = 'scheduled'
	WHERE id = $1 AND event_id = $2 AND status = 'draft'
	RETURNING id, event_id, venue_id, starts_at, ends_at,
	          sales_start_at, sales_end_at, status, created_at
	`
	rows, err = tx.Query(ctx, publishQuery, sessionID, eventID)
	if err != nil {
		return domain.Session{}, fmt.Errorf("publish session: %w", err)
	}
	saved, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Session])
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, fmt.Errorf("session is no longer draft: %w", core_errors.ErrConflict)
	}
	if err != nil {
		return domain.Session{}, fmt.Errorf("read published session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Session{}, fmt.Errorf("commit session publication: %w", err)
	}

	return saved, nil
}
