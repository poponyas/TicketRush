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

func (r *AuthRepository) GetOrganizerRole(ctx context.Context, organizerID, userID uuid.UUID) (domain.Role, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	const query = `SELECT role FROM organizer_memberships WHERE organizer_id=$1 AND user_id=$2`
	var role domain.Role
	if err := r.pool.Pool.QueryRow(ctx, query, organizerID, userID).Scan(&role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", coreerrors.ErrNotFound
		}
		return "", fmt.Errorf("get organizer role: %w", err)
	}
	return role, nil
}
