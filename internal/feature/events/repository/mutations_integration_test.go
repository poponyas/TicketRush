package events_repository_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	"github.com/poponyas/TicketRush/internal/feature/events/testutil"
	"github.com/stretchr/testify/require"
)

func TestListAccessPostgres(t *testing.T) {
	f := testutil.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	list, err := f.Repo.GetEvents(ctx, 20, 0, f.OrgID, f.UserID)
	require.NoError(t, err)
	require.Empty(t, list)
	_, err = f.Repo.GetEvents(ctx, 20, 0, f.OrgID, f.OtherUserID)
	require.ErrorIs(t, err, coreerrors.ErrForbidden)
	f.Exec(t, `INSERT INTO organizer_memberships(organizer_id,user_id,role) VALUES($1,$2,'controller')`, f.OrgID, f.OtherUserID)
	_, err = f.Repo.GetEvents(ctx, 20, 0, f.OrgID, f.OtherUserID)
	require.ErrorIs(t, err, coreerrors.ErrForbidden)
	// Equal dates must still have a stable descending UUID order.
	a, b := f.Event(t, f.OrgID, "A"), f.Event(t, f.OrgID, "B")
	f.Exec(t, `UPDATE events SET created_at=$1 WHERE organizer_id=$2`, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), f.OrgID)
	list, err = f.Repo.GetEvents(ctx, 20, 0, f.OrgID, f.UserID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	if a.ID.String() < b.ID.String() {
		a, b = b, a
	}
	require.Equal(t, a.ID, list[0].ID)
	require.Equal(t, b.ID, list[1].ID)
}

func TestUpdateEventPostgres(t *testing.T) {
	f := testutil.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	e := f.Event(t, f.OrgID, "Before")
	old := e
	e.Title = "After"
	e.Description = ""
	e.Category = "meetup"
	saved, err := f.Repo.UpdateEvent(ctx, e, f.UserID)
	require.NoError(t, err)
	require.Equal(t, e.Title, saved.Title)
	require.Empty(t, saved.Description)
	require.Equal(t, e.Category, saved.Category)
	require.Equal(t, old.CreatedAt, saved.CreatedAt)
	require.Equal(t, domain.EventDraft, saved.Status)
	require.True(t, saved.UpdatedAt.After(old.UpdatedAt))
	read, err := f.Repo.GetEvent(ctx, e.ID, f.OrgID, f.UserID)
	require.NoError(t, err)
	require.Equal(t, saved, read)
	old.Title = "Stale write"
	_, err = f.Repo.UpdateEvent(ctx, old, f.UserID)
	require.ErrorIs(t, err, coreerrors.ErrConflict)
	_, err = f.Repo.UpdateEvent(ctx, saved, f.OtherUserID)
	require.ErrorIs(t, err, coreerrors.ErrNotFound)
	f.Exec(t, `INSERT INTO organizer_memberships(organizer_id,user_id,role) VALUES($1,$2,'controller')`, f.OrgID, f.OtherUserID)
	_, err = f.Repo.UpdateEvent(ctx, saved, f.OtherUserID)
	require.ErrorIs(t, err, coreerrors.ErrNotFound)
	foreign := f.Event(t, f.OtherOrgID, "Foreign")
	foreign.OrganizerID = f.OrgID
	_, err = f.Repo.UpdateEvent(ctx, foreign, f.UserID)
	require.ErrorIs(t, err, coreerrors.ErrNotFound)
	missing := saved
	missing.ID = uuid.New()
	_, err = f.Repo.UpdateEvent(ctx, missing, f.UserID)
	require.ErrorIs(t, err, coreerrors.ErrNotFound)
	for _, status := range []domain.EventStatus{domain.EventPublished, domain.EventArchived} {
		f.Exec(t, `UPDATE events SET status=$1 WHERE id=$2`, status, e.ID)
		_, err = f.Repo.UpdateEvent(ctx, saved, f.UserID)
		require.ErrorIs(t, err, coreerrors.ErrConflict)
	}
	f.Exec(t, `UPDATE events SET status='draft' WHERE id=$1`, e.ID)
	// A failed SQL update must roll back without changing the row or holding locks.
	bad := saved
	bad.Title = ""
	_, err = f.Repo.UpdateEvent(ctx, bad, f.UserID)
	require.Error(t, err)
	read, err = f.Repo.GetEvent(ctx, e.ID, f.OrgID, f.UserID)
	require.NoError(t, err)
	require.Equal(t, saved, read)
}

func TestConcurrentUpdateEventPostgres(t *testing.T) {
	f := testutil.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	e := f.Event(t, f.OrgID, "Original")
	start := make(chan struct{})
	type outcome struct {
		event domain.Event
		err   error
	}
	results := make(chan outcome, 2)
	var wg sync.WaitGroup
	for _, title := range []string{"First", "Second"} {
		wg.Add(1)
		go func(title string) {
			defer wg.Done()
			<-start
			copy := e
			copy.Title = title
			saved, err := f.Repo.UpdateEvent(ctx, copy, f.UserID)
			results <- outcome{saved, err}
		}(title)
	}
	close(start)
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	var winner domain.Event
	for result := range results {
		if result.err == nil {
			success++
			winner = result.event
		} else {
			require.ErrorIs(t, result.err, coreerrors.ErrConflict)
			conflicts++
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, conflicts)
	read, err := f.Repo.GetEvent(ctx, e.ID, f.OrgID, f.UserID)
	require.NoError(t, err)
	require.Equal(t, winner, read)
}

func TestDeleteEventPostgres(t *testing.T) {
	f := testutil.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	e := f.Event(t, f.OrgID, "Delete")
	require.ErrorIs(t, f.Repo.DeleteEvent(ctx, e.ID, f.OrgID, f.OtherUserID), coreerrors.ErrNotFound)
	f.Exec(t, `INSERT INTO organizer_memberships(organizer_id,user_id,role) VALUES($1,$2,'controller')`, f.OrgID, f.OtherUserID)
	require.ErrorIs(t, f.Repo.DeleteEvent(ctx, e.ID, f.OrgID, f.OtherUserID), coreerrors.ErrNotFound)
	foreign := f.Event(t, f.OtherOrgID, "Foreign")
	require.ErrorIs(t, f.Repo.DeleteEvent(ctx, foreign.ID, f.OrgID, f.UserID), coreerrors.ErrNotFound)
	for _, status := range []domain.EventStatus{domain.EventPublished, domain.EventArchived} {
		f.Exec(t, `UPDATE events SET status=$1 WHERE id=$2`, status, e.ID)
		require.ErrorIs(t, f.Repo.DeleteEvent(ctx, e.ID, f.OrgID, f.UserID), coreerrors.ErrConflict)
	}
	f.Exec(t, `UPDATE events SET status='draft' WHERE id=$1`, e.ID)
	sessionID := f.Session(t, e.ID)
	require.ErrorIs(t, f.Repo.DeleteEvent(ctx, e.ID, f.OrgID, f.UserID), coreerrors.ErrConflict)
	var count int
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE id=$1`, sessionID).Scan(&count))
	require.Equal(t, 1, count)
	f.Exec(t, `DELETE FROM sessions WHERE id=$1`, sessionID)
	f.Exec(t, `INSERT INTO bookings(id,event_id,user_id,quantity) VALUES($1,$2,'legacy-user',1)`, uuid.New(), e.ID)
	require.ErrorIs(t, f.Repo.DeleteEvent(ctx, e.ID, f.OrgID, f.UserID), coreerrors.ErrConflict)
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT count(*) FROM bookings WHERE event_id=$1`, e.ID).Scan(&count))
	require.Equal(t, 1, count)
	f.Exec(t, `DELETE FROM bookings WHERE event_id=$1`, e.ID)
	require.NoError(t, f.Repo.DeleteEvent(ctx, e.ID, f.OrgID, f.UserID))
	_, err := f.Repo.GetEvent(ctx, e.ID, f.OrgID, f.UserID)
	require.ErrorIs(t, err, coreerrors.ErrNotFound)
	require.ErrorIs(t, f.Repo.DeleteEvent(ctx, e.ID, f.OrgID, f.UserID), coreerrors.ErrNotFound)
}
