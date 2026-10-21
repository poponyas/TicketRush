package events_repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/poponyas/TicketRush/internal/core/domain"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	repository "github.com/poponyas/TicketRush/internal/feature/events/repository"
	"github.com/stretchr/testify/require"
)

func TestGetPublicEventsPostgres(t *testing.T) {
	url := os.Getenv("TICKETRUSH_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TICKETRUSH_TEST_DATABASE_URL to a disposable database with migrations")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	t.Cleanup(admin.Close)

	// The public catalog has no organizer scope. A private schema prevents seeded
	// events and other tests from changing the expected pages. Copy the migrated
	// table's columns, defaults, checks and indexes; this read test needs no FKs.
	schema := "public_events_test_" + uuid.New().String()
	quoted := pgx.Identifier{schema}.Sanitize()
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+quoted+" CASCADE")
		if err != nil {
			t.Errorf("cleanup public events schema: %v", err)
		}
	})
	_, err = admin.Exec(ctx, "CREATE TABLE "+quoted+".events (LIKE events INCLUDING ALL)")
	require.NoError(t, err)
	config, err := pgxpool.ParseConfig(url)
	require.NoError(t, err)
	config.ConnConfig.RuntimeParams["search_path"] = quoted
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	repo := repository.NewEventsRepo(&postgres.Pool{Pool: pool, OpTimeout: 5 * time.Second})

	empty, err := repo.GetPublicEvents(ctx, 20, 0)
	require.NoError(t, err)
	require.Empty(t, empty)

	older := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	tieLow := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	tieHigh := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	org, otherOrg := uuid.New(), uuid.New()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for _, row := range []struct {
		id, org uuid.UUID
		status  domain.EventStatus
		at      time.Time
	}{
		{older, org, domain.EventPublished, base},
		{tieLow, org, domain.EventPublished, base.Add(time.Hour)},
		{tieHigh, otherOrg, domain.EventPublished, base.Add(time.Hour)},
		// Hidden records sort before all published ones if the WHERE is omitted.
		{uuid.New(), org, domain.EventDraft, base.Add(2 * time.Hour)},
		{uuid.New(), otherOrg, domain.EventArchived, base.Add(3 * time.Hour)},
	} {
		_, err := pool.Exec(ctx, `INSERT INTO events
   (id,organizer_id,name,description,category,poster_key,status,created_at,updated_at)
   VALUES($1,$2,'Go Live','Description','conference','posters/test.png',$3,$4,$4)`, row.id, row.org, row.status, row.at)
		require.NoError(t, err)
	}
	first, err := repo.GetPublicEvents(ctx, 2, 0)
	require.NoError(t, err)
	require.Len(t, first, 2)
	require.Equal(t, tieHigh, first[0].ID)
	require.Equal(t, tieLow, first[1].ID)
	require.Equal(t, otherOrg, first[0].OrganizerID)
	for _, event := range first {
		require.Equal(t, domain.EventPublished, event.Status)
		require.Equal(t, "Go Live", event.Title)
		require.Equal(t, "Description", event.Description)
		require.Equal(t, "conference", event.Category)
		require.Equal(t, "posters/test.png", event.PosterKey)
		require.True(t, base.Add(time.Hour).Equal(event.CreatedAt))
		require.True(t, event.CreatedAt.Equal(event.UpdatedAt))
	}
	second, err := repo.GetPublicEvents(ctx, 2, 2)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, older, second[0].ID)
	beyond, err := repo.GetPublicEvents(ctx, 2, 3)
	require.NoError(t, err)
	require.Empty(t, beyond)
}
