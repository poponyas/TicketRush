package auth_repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *AuthRepository) CreateSession(
	ctx context.Context,
	sessionID uuid.UUID,
	userID uuid.UUID,
	refreshHash string,
	expiresAt time.Time,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()

	query := `
	INSERT INTO auth_sessions(id, user_id, refresh_token_hash, expires_at)
	VALUES ($1, $2, $3, $4);
	`

	if _, err := r.pool.Pool.Exec(
		ctx,
		query,
		sessionID,
		userID,
		refreshHash,
		expiresAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "auth_session_user_id_key" {
			return fmt.Errorf("foreing key violates error: %w", core_errors.ErrConflict)
		}
		return fmt.Errorf("exec error: %w", err)
	}

	return nil
}
