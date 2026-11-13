package core_clickhouse

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// OpenClient is lazy: API startup does not depend on analytics availability.
func OpenClient(cfg Config) (driver.Conn, error) {
	if cfg.Timeout <= 0 {
		return nil, fmt.Errorf("clickhouse timeout must be positive")
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.Addr},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		DialTimeout: cfg.Timeout,
		ReadTimeout: cfg.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse tune config vars: %w", err)
	}

	return conn, nil
}

// NewClient additionally checks connectivity for the dedicated analytics worker.
func NewClient(ctx context.Context, cfg Config) (driver.Conn, error) {
	conn, err := OpenClient(cfg)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}

	return conn, nil
}
