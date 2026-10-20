package events_repository_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	repository "github.com/poponyas/TicketRush/internal/feature/events/repository"
	"github.com/stretchr/testify/require"
)

func TestGetEventsPostgres(t *testing.T) {
	url := os.Getenv("TICKETRUSH_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TICKETRUSH_TEST_DATABASE_URL for integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	defer pool.Close()

	userID, orgID, otherOrgID := uuid.New(), uuid.New(), uuid.New()
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := pool.Exec(cleanupCtx, `DELETE FROM events WHERE organizer_id IN ($1,$2)`, orgID, otherOrgID)
		if err != nil {
			t.Errorf("cleanup events: %v", err)
		}
		_, err = pool.Exec(cleanupCtx, `DELETE FROM organizer_memberships WHERE organizer_id IN ($1,$2)`, orgID, otherOrgID)
		if err != nil {
			t.Errorf("cleanup memberships: %v", err)
		}
		_, err = pool.Exec(cleanupCtx, `DELETE FROM organizers WHERE id IN ($1,$2)`, orgID, otherOrgID)
		if err != nil {
			t.Errorf("cleanup organizers: %v", err)
		}
		_, err = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1`, userID)
		if err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	}()
	_, err = pool.Exec(ctx, `INSERT INTO users(id,email,password_hash) VALUES($1,$2,'unused-test-hash')`, userID, userID.String()+"@example.test")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO organizers(id,name) VALUES($1,'Integration')`, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO organizer_memberships(organizer_id,user_id,role) VALUES($1,$2,'organizer')`, orgID, userID)
	require.NoError(t, err)

	repo := repository.NewEventsRepo(&postgres.Pool{Pool: pool, OpTimeout: 5 * time.Second})
	// IDs are ordered oldest to newest; fixed dates keep the test deterministic.
	eventIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	baseTime := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	for i, eventID := range eventIDs {
		_, err = pool.Exec(
			ctx,
			`INSERT INTO events(id,organizer_id,name,created_at) VALUES($1,$2,$3,$4)`,
			eventID,
			orgID,
			fmt.Sprintf("Own event %d", i),
			baseTime.Add(time.Duration(i)*time.Hour),
		)
		require.NoError(t, err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO organizers(id,name) VALUES($1,$2)`, otherOrgID, "Other organizer")
	require.NoError(t, err)
	otherEventID := uuid.New()
	// A foreign event sorts first if the ownership filter is accidentally removed.
	_, err = pool.Exec(ctx, `INSERT INTO events(id,organizer_id,name,created_at) VALUES($1,$2,$3,$4)`,
		otherEventID, otherOrgID, "Foreign event", baseTime.Add(3*time.Hour))
	require.NoError(t, err)
	// The same user belongs to both organizers: user_id alone must not scope the list.
	_, err = pool.Exec(ctx, `INSERT INTO organizer_memberships(organizer_id,user_id,role) VALUES($1,$2,'organizer')`, otherOrgID, userID)
	require.NoError(t, err)

	events, err := repo.GetEvents(ctx, 2, 0, orgID, userID)
	require.NoError(t, err)
	require.Len(t, events, 2)
	require.Equal(t, eventIDs[2], events[0].ID)
	require.Equal(t, eventIDs[1], events[1].ID)
	for _, event := range events {
		require.Equal(t, orgID, event.OrganizerID)
	}

	events, err = repo.GetEvents(ctx, 2, 2, orgID, userID)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, eventIDs[0], events[0].ID)
	require.Equal(t, orgID, events[0].OrganizerID)

	events, err = repo.GetEvents(ctx, 2, 3, orgID, userID)
	require.NoError(t, err)
	require.Empty(t, events)
}
