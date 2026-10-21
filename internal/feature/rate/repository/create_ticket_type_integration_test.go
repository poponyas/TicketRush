package ticket_type_repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	"github.com/poponyas/TicketRush/internal/feature/events/testutil"
	repository "github.com/poponyas/TicketRush/internal/feature/rate/repository"
	"github.com/stretchr/testify/require"
)

func TestCreateTicketTypePostgres(t *testing.T) {
	f := testutil.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	event := f.Event(t, f.OrgID, "Ticket types")
	otherEvent := f.Event(t, f.OrgID, "Other event")
	sessionID := f.Session(t, event.ID)
	repo := repository.NewTicketTypePool(&postgres.Pool{Pool: f.Pool, OpTimeout: 5 * time.Second})
	price, err := domain.NewMoney("1500.25", "RUB")
	require.NoError(t, err)
	// Construct the repository input directly: domain.NewTicketType is a separate exercise.
	draft := domain.TicketType{ID: uuid.New(), SessionID: sessionID, Name: "Standard", Price: price, Capacity: 100, Available: 100, CreatedAt: time.Now().UTC()}
	saved, err := repo.CreateTicketType(ctx, draft, event.ID, f.OrgID, f.UserID)
	require.NoError(t, err)
	require.Equal(t, draft.ID, saved.ID)
	require.Equal(t, sessionID, saved.SessionID)
	require.Equal(t, draft.Name, saved.Name)
	require.True(t, price.Amount.Equal(saved.Price.Amount))
	require.Equal(t, "RUB", saved.Price.Currency)
	require.Equal(t, 100, saved.Capacity)
	require.Equal(t, 100, saved.Available)
	require.Zero(t, saved.Held)
	require.Zero(t, saved.Sold)
	require.False(t, saved.CreatedAt.IsZero())
	var storedPrice, storedCurrency string
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT price::text,currency FROM ticket_types WHERE id=$1`, saved.ID).Scan(&storedPrice, &storedCurrency))
	require.Equal(t, "1500.25", storedPrice)
	require.Equal(t, "RUB", storedCurrency)
	// Zero-priced tickets are valid too.
	free := draft
	free.ID = uuid.New()
	free.Price, err = domain.NewMoney("0", "RUB")
	require.NoError(t, err)
	freeSaved, err := repo.CreateTicketType(ctx, free, event.ID, f.OrgID, f.UserID)
	require.NoError(t, err)
	require.True(t, freeSaved.Price.Amount.IsZero())
	reject := func(eventID, orgID, userID uuid.UUID, want error) {
		t.Helper()
		candidate := draft
		candidate.ID = uuid.New()
		_, err := repo.CreateTicketType(ctx, candidate, eventID, orgID, userID)
		require.ErrorIs(t, err, want)
		var count int
		require.NoError(t, f.Pool.QueryRow(ctx, `SELECT count(*) FROM ticket_types WHERE id=$1`, candidate.ID).Scan(&count))
		require.Zero(t, count)
	}
	reject(otherEvent.ID, f.OrgID, f.UserID, coreerrors.ErrNotFound)
	reject(event.ID, f.OtherOrgID, f.UserID, coreerrors.ErrNotFound)
	reject(event.ID, f.OrgID, f.OtherUserID, coreerrors.ErrNotFound)
	f.Exec(t, `INSERT INTO organizer_memberships(organizer_id,user_id,role) VALUES($1,$2,'controller')`, f.OrgID, f.OtherUserID)
	reject(event.ID, f.OrgID, f.OtherUserID, coreerrors.ErrNotFound)
	f.Exec(t, `UPDATE events SET status='published' WHERE id=$1`, event.ID)
	reject(event.ID, f.OrgID, f.UserID, coreerrors.ErrConflict)
	f.Exec(t, `UPDATE events SET status='draft' WHERE id=$1`, event.ID)
	for _, status := range []string{"scheduled", "cancelled", "completed"} {
		f.Exec(t, `UPDATE sessions SET status=$1 WHERE id=$2`, status, sessionID)
		reject(event.ID, f.OrgID, f.UserID, coreerrors.ErrConflict)
	}
	f.Exec(t, `UPDATE sessions SET status='draft' WHERE id=$1`, sessionID)
	invalid := draft
	invalid.ID = uuid.New()
	invalid.Available = 101
	_, err = repo.CreateTicketType(ctx, invalid, event.ID, f.OrgID, f.UserID)
	require.Error(t, err)
	var count int
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT count(*) FROM ticket_types WHERE id=$1`, invalid.ID).Scan(&count))
	require.Zero(t, count)
	// A failed INSERT must roll back without preventing later valid operations.
	next := draft
	next.ID = uuid.New()
	_, err = repo.CreateTicketType(ctx, next, event.ID, f.OrgID, f.UserID)
	require.NoError(t, err)
}
