package venue_repository

import core_postgres_pool "github.com/poponyas/TicketRush/internal/core/repository/postgres"

type VenueRepo struct {
	pool *core_postgres_pool.Pool
}

func NewVenueRepository(
	pool *core_postgres_pool.Pool,
) *VenueRepo {
	return &VenueRepo{
		pool: pool,
	}
}
