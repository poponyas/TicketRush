package venue_repository

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *VenueRepo) CheckVenueOwnerAccess(ctx context.Context, user uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	var allowed bool
	if err := r.pool.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_roles WHERE user_id=$1 AND role='ownerVenue')`, user).Scan(&allowed); err != nil {
		return fmt.Errorf("venue owner role: %w", err)
	}
	if !allowed {
		return coreerrors.ErrForbidden
	}
	return nil
}
