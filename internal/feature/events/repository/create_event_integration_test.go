package events_repository_test

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	repository "github.com/poponyas/TicketRush/internal/feature/events/repository"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

// Run only against a disposable database with all migrations applied.
func TestCreateEventPostgres(t *testing.T) {
	url := os.Getenv("TICKETRUSH_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TICKETRUSH_TEST_DATABASE_URL for integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	defer pool.Close()
	userID, orgID := uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO users(id,email,password_hash) VALUES($1,$2,'unused-test-hash')`, userID, userID.String()+"@example.test")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO organizers(id,name) VALUES($1,'Integration')`, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO organizer_memberships(organizer_id,user_id,role) VALUES($1,$2,'organizer')`, orgID, userID)
	require.NoError(t, err)
	// Restore our own fixtures; never truncate shared tables.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM events WHERE organizer_id=$1`, orgID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM organizer_memberships WHERE organizer_id=$1`, orgID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM organizers WHERE id=$1`, orgID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1`, userID)
	}()
	repo := repository.NewEventsRepo(&postgres.Pool{Pool: pool, OpTimeout: 5 * time.Second})
	draft := domain.NewEvent(orgID, "Go Live", "Description", "conference")
	saved, err := repo.CreateEvent(ctx, draft, userID)
	require.NoError(t, err)
	require.Equal(t, draft.ID, saved.ID)
	require.Equal(t, domain.EventDraft, saved.Status)
	require.False(t, saved.CreatedAt.IsZero())
	require.Equal(t, saved.CreatedAt, saved.UpdatedAt)
	var name string
	require.NoError(t, pool.QueryRow(ctx, `SELECT name FROM events WHERE id=$1`, saved.ID).Scan(&name))
	require.Equal(t, "Go Live", name)
	other := domain.NewEvent(orgID, "Unauthorized", "", "")
	_, err = repo.CreateEvent(ctx, other, uuid.New())
	require.ErrorIs(t, err, coreerrors.ErrForbidden)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM events WHERE id=$1`, other.ID).Scan(&count))
	require.Zero(t, count)
	// Verify pgx/decimal interoperability without float conversion.
	amount, err := decimal.NewFromString("123.45")
	require.NoError(t, err)
	var scanned decimal.Decimal
	require.NoError(t, pool.QueryRow(ctx, `SELECT $1::NUMERIC(18,2)`, amount).Scan(&scanned))
	require.True(t, amount.Equal(scanned))
}
