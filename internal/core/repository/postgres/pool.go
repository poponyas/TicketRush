package core_postgres_pool

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Pool struct {
	Pool *pgxpool.Pool

	OpTimeout time.Duration
}

// NewPool creates a pool and checks connectivity within OperationTimeout.
// Query timeouts are controlled separately by the caller's context.
func NewPool(ctx context.Context, cfg Config) (*Pool, error) {
	if cfg.OperationTimeout <= 0 {
		return nil, fmt.Errorf("postgres connection timeout must be positive")
	}
	poolConfig, err := pgxpool.ParseConfig(cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}
	initCtx, cancel := context.WithTimeout(ctx, cfg.OperationTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(initCtx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(initCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Pool{
		Pool:      pool,
		OpTimeout: cfg.OperationTimeout,
	}, nil
}

func (p *Pool) OperationTimeout() time.Duration {
	return p.OpTimeout
}
