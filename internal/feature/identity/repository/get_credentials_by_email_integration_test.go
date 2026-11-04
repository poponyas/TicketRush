package auth_repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	repository "github.com/poponyas/TicketRush/internal/feature/identity/repository"
	"github.com/stretchr/testify/require"
)

func TestGetCredentialsByEmailWithoutPasswordPostgres(t *testing.T) {
	url := os.Getenv("TICKETRUSH_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TICKETRUSH_TEST_DATABASE_URL to a disposable migrated database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	userID := uuid.New()
	email := userID.String() + "@passwordless.example.test"
	_, err = pool.Exec(ctx, `INSERT INTO users(id,email) VALUES($1,$2)`, userID, email)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, err := pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1`, userID)
		require.NoError(t, err)
	})

	repo := repository.NewAuthRepo(&postgres.Pool{Pool: pool, OpTimeout: 5 * time.Second})
	_, err = repo.GetCredentialsByEmail(ctx, email)
	require.ErrorIs(t, err, coreerrors.ErrNotFound,
		"a legacy account without password credentials must follow the normal unauthenticated login path")
}
