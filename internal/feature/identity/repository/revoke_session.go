package auth_repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

func (r *AuthRepository) RevokeSession(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()

	const query = `
	UPDATE auth_sessions
	SET revoked_at = COALESCE(revoked_at, NOW())
	WHERE id = $1 AND user_id = $2;
	`

	_, err := r.pool.Pool.Exec(ctx, query, sessionID, userID)
	if err != nil {
		return fmt.Errorf("revoke auth session: %w", err)
	}

	return nil
}
