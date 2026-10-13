// Package testutil isolates the shared venue catalog in a temporary test schema.
package testutil

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	repository "github.com/poponyas/TicketRush/internal/feature/venue/repository"
	"github.com/stretchr/testify/require"
)

func Database(t *testing.T) (*pgxpool.Pool, *repository.VenueRepo) {
	t.Helper()
	url := os.Getenv("TICKETRUSH_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TICKETRUSH_TEST_DATABASE_URL to a disposable migrated database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	root, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	t.Cleanup(root.Close)
	schema := pgx.Identifier{"venue_test_" + strings.ReplaceAll(uuid.New().String(), "-", "")}.Sanitize()
	_, err = root.Exec(ctx, `CREATE SCHEMA `+schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := root.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`)
		if err != nil {
			t.Errorf("cleanup venue schema: %v", err)
		}
	})
	// Clone the migrated definition so constraints/types match the application table.
	_, err = root.Exec(ctx, `CREATE TABLE `+schema+`.venues (LIKE public.venues INCLUDING ALL)`)
	require.NoError(t, err)
	cfg, err := pgxpool.ParseConfig(url)
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx))
	return pool, repository.NewVenueRepository(&postgres.Pool{Pool: pool, OpTimeout: 5 * time.Second})
}
