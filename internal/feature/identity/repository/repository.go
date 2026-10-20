package auth_repository

import core_postgres_pool "github.com/poponyas/TicketRush/internal/core/repository/postgres"

type AuthRepository struct {
	pool *core_postgres_pool.Pool
}

func NewAuthRepo(
	pool *core_postgres_pool.Pool,
) *AuthRepository {
	return &AuthRepository{
		pool: pool,
	}
}
