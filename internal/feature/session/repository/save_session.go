package session_repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/poponyas/TicketRush/internal/core/domain"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *SessionRepository) CreateSession(ctx context.Context,
	session domain.Session,
	organizerID uuid.UUID,
	userID uuid.UUID,
) (domain.Session, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()

	tx, err := r.pool.Pool.Begin(ctx)
	if err != nil {
		return domain.Session{}, fmt.Errorf("can't begin transaction: %w", err)
	}
	defer func() {
		rollbackCtx, rollbackCancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer rollbackCancel()

		_ = tx.Rollback(rollbackCtx)
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
	INSERT INTO sessions (
		id, event_id, venue_id,
		starts_at, ends_at,
		sales_start_at, sales_end_at,
		status, created_at
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	RETURNING
		id, event_id, venue_id,
		starts_at, ends_at,
		sales_start_at, sales_end_at,
		status, created_at
`

	rows, err := tx.Query(
		ctx,
		query,
		session.ID,
		session.EventID,
		session.VenueID,
		session.StartsAt,
		session.EndsAt,
		session.SalesStartAt,
		session.SalesEndAt,
		session.Status,
		session.CreatedAt,
	)
	if err != nil {
		return domain.Session{}, sessionInsertError(err)
	}
	saved, err := pgx.CollectExactlyOneRow(
		rows,
		pgx.RowToStructByName[domain.Session],
	)
	if err != nil {
		return domain.Session{}, sessionInsertError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Session{}, fmt.Errorf("commit session: %w", err)
	}

	return saved, nil
}

// Query errors may be reported while starting the query or while reading rows.
func sessionInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "sessions_venue_id_fkey" {
		return fmt.Errorf("venue not found: %w", core_errors.ErrNotFound)
	}
	return fmt.Errorf("insert session: %w", err)
}
