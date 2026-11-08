package outbox_repository

import core_postgres_pool "github.com/poponyas/TicketRush/internal/core/repository/postgres"

type OutboxRepository struct {
	Pool *core_postgres_pool.Pool
}

func NewOutboxRepository(
	pool *core_postgres_pool.Pool,
) *OutboxRepository {
	return &OutboxRepository{
		Pool: pool,
	}
}
