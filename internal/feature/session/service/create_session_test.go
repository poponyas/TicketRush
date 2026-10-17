package session_service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/session/service"
	"github.com/stretchr/testify/require"
)

type sessionRepoStub struct {
	update func(context.Context, domain.Session, domain.Session, uuid.UUID, uuid.UUID) (domain.Session, error)
	create func(context.Context, domain.Session, uuid.UUID, uuid.UUID) (domain.Session, error)
	get    func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Session, error)
	list   func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int, int) ([]domain.Session, error)
}

func (r *sessionRepoStub) GetSession(ctx context.Context, event, session, org, user uuid.UUID) (domain.Session, error) {
	return r.get(ctx, event, session, org, user)
}
func (r *sessionRepoStub) GetSessions(ctx context.Context, event, org, user uuid.UUID, limit, offset int) ([]domain.Session, error) {
	return r.list(ctx, event, org, user, limit, offset)
}

func (r *sessionRepoStub) CreateSession(ctx context.Context, s domain.Session, org, user uuid.UUID) (domain.Session, error) {
	return r.create(ctx, s, org, user)
}

func TestCreateSessionService(t *testing.T) {
	org, user, event := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	input := service.CreateSessionInput{VenueID: uuid.New(), StartsAt: now.Add(48 * time.Hour), SalesStartAt: now.Add(-time.Hour), SalesEndAt: now.Add(24 * time.Hour)}
	called := 0
	r := &sessionRepoStub{create: func(ctx context.Context, s domain.Session, orgID, userID uuid.UUID) (domain.Session, error) {
		called++
		require.Equal(t, org, orgID)
		require.Equal(t, user, userID)
		require.Equal(t, event, s.EventID)
		require.Equal(t, input.VenueID, s.VenueID)
		require.Equal(t, input.StartsAt, s.StartsAt)
		require.Equal(t, input.SalesStartAt, s.SalesStartAt)
		require.Equal(t, input.SalesEndAt, s.SalesEndAt)
		require.Nil(t, s.EndsAt)
		require.Equal(t, domain.SessionDraft, s.Status)
		require.NotEqual(t, uuid.Nil, s.ID)
		require.False(t, s.CreatedAt.Before(now))
		return s, nil
	}}
	_, err := service.NewSessionService(r).CreateSession(context.Background(), input, org, user, event)
	require.NoError(t, err)
	require.Equal(t, 1, called)
	for _, tc := range []struct {
		name   string
		change func(*service.CreateSessionInput)
	}{
		{"missing venue", func(i *service.CreateSessionInput) { i.VenueID = uuid.Nil }},
		{"missing sales start", func(i *service.CreateSessionInput) { i.SalesStartAt = time.Time{} }},
		{"past session", func(i *service.CreateSessionInput) {
			i.StartsAt = now.Add(-time.Hour)
			i.SalesEndAt = now.Add(-2 * time.Hour)
			i.SalesStartAt = now.Add(-3 * time.Hour)
		}},
		{"sales closed", func(i *service.CreateSessionInput) { i.SalesEndAt = now.Add(-time.Minute) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := input
			tc.change(&bad)
			_, err := service.NewSessionService(&sessionRepoStub{}).CreateSession(context.Background(), bad, org, user, event)
			require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
		})
	}
	for _, ids := range [][3]uuid.UUID{{uuid.Nil, user, event}, {org, uuid.Nil, event}, {org, user, uuid.Nil}} {
		_, err := service.NewSessionService(&sessionRepoStub{}).CreateSession(context.Background(), input, ids[0], ids[1], ids[2])
		require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	}
	r.create = func(context.Context, domain.Session, uuid.UUID, uuid.UUID) (domain.Session, error) {
		return domain.Session{}, fmt.Errorf("storage: %w", coreerrors.ErrNotFound)
	}
	_, err = service.NewSessionService(r).CreateSession(context.Background(), input, org, user, event)
	require.ErrorIs(t, err, coreerrors.ErrNotFound)
}

func (r *sessionRepoStub) UpdateSession(ctx context.Context, updated, expected domain.Session, org, user uuid.UUID) (domain.Session, error) {
	return r.update(ctx, updated, expected, org, user)
}
