package ticket_type_repository

import (
	core_postgres_pool "github.com/poponyas/TicketRush/internal/core/repository/postgres"
)

type TicketTypeRepo struct {
	pool *core_postgres_pool.Pool
}

func NewTicketTypePool(
	pool *core_postgres_pool.Pool,
) *TicketTypeRepo {
	return &TicketTypeRepo{
		pool: pool,
	}
}
