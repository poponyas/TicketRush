package auth_repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *AuthRepository) GetActiveSession(ctx context.Context, sessionID, userID uuid.UUID) (domain.AuthSession, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	const query = `SELECT id,user_id,expires_at,revoked_at,created_at
	FROM auth_sessions
	WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL AND expires_at>NOW()`
	var session domain.AuthSession
	err := r.pool.Pool.QueryRow(ctx, query, sessionID, userID).Scan(
		&session.ID, &session.UserID, &session.ExpiresAt, &session.RevokedAt, &session.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AuthSession{}, coreerrors.ErrNotFound
	}
	if err != nil {
		return domain.AuthSession{}, fmt.Errorf("get active auth session: %w", err)
	}
	return session, nil
}
