package events_repository

import core_postgres_pool "github.com/poponyas/TicketRush/internal/core/repository/postgres"

type EventsRepository struct {
	pool *core_postgres_pool.Pool
}

func NewEventsRepo(
	pool *core_postgres_pool.Pool,
) *EventsRepository {
	return &EventsRepository{
		pool: pool,
	}
}
