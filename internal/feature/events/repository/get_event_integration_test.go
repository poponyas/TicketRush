package events_repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/poponyas/TicketRush/internal/core/domain"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	repository "github.com/poponyas/TicketRush/internal/feature/events/repository"
	"github.com/stretchr/testify/require"
)

func TestGetEventPostgres(t *testing.T) {
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
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM events WHERE organizer_id=$1`, orgID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM organizer_memberships WHERE organizer_id=$1`, orgID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM organizers WHERE id=$1`, orgID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1`, userID)
	}()
	_, err = pool.Exec(ctx, `INSERT INTO users(id,email,password_hash) VALUES($1,$2,'unused-test-hash')`, userID, userID.String()+"@example.test")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO organizers(id,name) VALUES($1,'Integration')`, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO organizer_memberships(organizer_id,user_id,role) VALUES($1,$2,'organizer')`, orgID, userID)
	require.NoError(t, err)

	repo := repository.NewEventsRepo(&postgres.Pool{Pool: pool, OpTimeout: 5 * time.Second})
	draft := domain.NewEvent(orgID, "Go Live", "Description", "conference")
	saved, err := repo.CreateEvent(ctx, draft, userID)
	require.NoError(t, err)
	require.Equal(t, draft.ID, saved.ID)
	require.Equal(t, domain.EventDraft, saved.Status)
	require.False(t, saved.CreatedAt.IsZero())
	require.Equal(t, saved.CreatedAt, saved.UpdatedAt)
	event, err := repo.GetEvent(ctx, draft.ID, orgID, userID)
	require.NoError(t, err)
	require.Equal(t, saved.ID, event.ID)
	require.Equal(t, orgID, event.OrganizerID)
	require.Equal(t, draft.Title, event.Title)
	require.Equal(t, domain.EventDraft, event.Status)
	require.False(t, event.CreatedAt.IsZero())
	require.Equal(t, saved.CreatedAt, event.CreatedAt)

	otherUserID := uuid.New()

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cancel()

		_, err := pool.Exec(
			cleanupCtx,
			`DELETE FROM organizer_memberships
		 WHERE organizer_id=$1 AND user_id=$2`,
			orgID,
			otherUserID,
		)
		if err != nil {
			t.Errorf("cleanup membership: %v", err)
		}

		_, err = pool.Exec(
			cleanupCtx,
			`DELETE FROM users WHERE id=$1`,
			otherUserID,
		)
		if err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	}()

	_, err = pool.Exec(
		ctx,
		`INSERT INTO users(id,email,password_hash) VALUES($1,$2,'unused-test-hash')`,
		otherUserID,
		otherUserID.String()+"@example.test",
	)
	require.NoError(t, err)

	_, err = repo.GetEvent(ctx, draft.ID, orgID, otherUserID)
	require.ErrorIs(t, err, core_errors.ErrNotFound)

	_, err = pool.Exec(
		ctx,
		`INSERT INTO organizer_memberships(organizer_id,user_id,role)
	 VALUES($1,$2,'controller')`,
		orgID,
		otherUserID,
	)
	require.NoError(t, err)

	_, err = repo.GetEvent(ctx, draft.ID, orgID, otherUserID)
	require.ErrorIs(t, err, core_errors.ErrNotFound)

	otherOrgID := uuid.New()
	otherEventID := uuid.New()

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cancel()

		_, cleanupErr := pool.Exec(
			cleanupCtx,
			`DELETE FROM events WHERE id=$1`,
			otherEventID,
		)
		if cleanupErr != nil {
			t.Errorf("event cleanup: %v", cleanupErr)
		}

		_, cleanupErr = pool.Exec(
			cleanupCtx,
			`DELETE FROM organizers WHERE id=$1`,
			otherOrgID,
		)
		if cleanupErr != nil {
			t.Errorf("organizer cleanup: %v", cleanupErr)
		}
	}()

	_, err = pool.Exec(
		ctx,
		`INSERT INTO organizers(id,name) VALUES($1,$2)`,
		otherOrgID,
		"Other organizer",
	)
	require.NoError(t, err)

	_, err = pool.Exec(
		ctx,
		`INSERT INTO events(id,organizer_id,name) VALUES($1,$2,$3)`,
		otherEventID,
		otherOrgID,
		"Other event",
	)
	require.NoError(t, err)

	// Своя организация, но чужое мероприятие.
	_, err = repo.GetEvent(ctx, otherEventID, orgID, userID)
	require.ErrorIs(t, err, core_errors.ErrNotFound)

	// Чужая организация, в которой пользователь не состоит.
	_, err = repo.GetEvent(ctx, otherEventID, otherOrgID, userID)
	require.ErrorIs(t, err, core_errors.ErrNotFound)
}
