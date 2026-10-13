package venue_repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *VenueRepo) GetVenue(ctx context.Context, id uuid.UUID) (domain.Venue, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	rows, err := r.pool.Pool.Query(ctx, `SELECT id,name,city,address FROM venues WHERE id=$1`, id)
	if err != nil {
		return domain.Venue{}, fmt.Errorf("query venue: %w", err)
	}
	venue, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Venue])
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Venue{}, coreerrors.ErrNotFound
	}
	if err != nil {
		return domain.Venue{}, fmt.Errorf("read venue: %w", err)
	}
	return venue, nil
}
