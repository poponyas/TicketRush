package auth_repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *AuthRepository) RotateRefreshToken(
	ctx context.Context,
	sessionID uuid.UUID,
	userID uuid.UUID,
	oldHash string,
	newHash string,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()

	const query = `
		UPDATE auth_sessions
		SET refresh_token_hash = $1
		WHERE id = $2
		  AND user_id = $3
		  AND refresh_token_hash = $4
		  AND revoked_at IS NULL
		  AND expires_at > NOW()
	`

	result, err := r.pool.Pool.Exec(
		ctx,
		query,
		newHash,
		sessionID,
		userID,
		oldHash,
	)
	if err != nil {
		return fmt.Errorf("rotate refresh token: %w", err)
	}

	if result.RowsAffected() == 0 {
		return core_errors.ErrNotFound
	}

	return nil
}
