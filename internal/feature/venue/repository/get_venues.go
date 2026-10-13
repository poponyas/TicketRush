package venue_repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
)

func (r *VenueRepo) GetVenues(ctx context.Context, limit, offset int) ([]domain.Venue, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	const query = `SELECT id,name,city,address FROM venues ORDER BY name ASC,id ASC LIMIT $1 OFFSET $2`
	rows, err := r.pool.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query venues: %w", err)
	}
	venues, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.Venue])
	if err != nil {
		return nil, fmt.Errorf("read venues: %w", err)
	}
	return venues, nil
}
