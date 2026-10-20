package auth_repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *AuthRepository) GetActiveSessionByRefreshHash(
	ctx context.Context,
	hash string,
) (domain.AuthSession, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()

	const query = `
		SELECT id, user_id, expires_at, created_at
		FROM auth_sessions
		WHERE refresh_token_hash = $1
		  AND revoked_at IS NULL
		  AND expires_at > NOW()
	`

	var session domain.AuthSession

	err := r.pool.Pool.QueryRow(ctx, query, hash).Scan(
		&session.ID,
		&session.UserID,
		&session.ExpiresAt,
		&session.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AuthSession{}, core_errors.ErrNotFound
		}

		return domain.AuthSession{}, fmt.Errorf(
			"get active session by refresh hash: %w", err,
		)
	}

	return session, nil
}
