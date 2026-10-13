package venue_repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
)

func (r *VenueRepo) CreateVenue(ctx context.Context, venue domain.Venue) (domain.Venue, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()

	const query = `
	INSERT INTO venues(id, name, city, address)
	VALUES ($1, $2, $3, $4)
	RETURNING id, name, city, address;
	`

	rows, err := r.pool.Pool.Query(ctx, query, venue.ID, venue.Name, venue.City, venue.Address)
	if err != nil {
		return domain.Venue{}, fmt.Errorf("insert venue: %w", err)
	}

	response, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Venue])
	if err != nil {
		return domain.Venue{}, fmt.Errorf("read inserted venue: %w", err)
	}

	return response, nil
}
