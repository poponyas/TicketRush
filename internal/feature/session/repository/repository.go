package session_repository

import core_postgres_pool "github.com/poponyas/TicketRush/internal/core/repository/postgres"

type SessionRepository struct {
	pool *core_postgres_pool.Pool
}

func NewSessionRepository(
	pool *core_postgres_pool.Pool,
) *SessionRepository {
	return &SessionRepository{
		pool: pool,
	}
}
