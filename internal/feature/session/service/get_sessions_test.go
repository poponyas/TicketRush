package session_service_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/session/service"
	"github.com/stretchr/testify/require"
)

func sessionNumber(n int) *int { return &n }

func TestSessionReadValidation(t *testing.T) {
	event, id, org, user := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, ids := range [][4]uuid.UUID{{uuid.Nil, id, org, user}, {event, uuid.Nil, org, user}, {event, id, uuid.Nil, user}, {event, id, org, uuid.Nil}} {
		_, err := service.NewSessionService(&sessionRepoStub{}).GetSession(context.Background(), ids[0], ids[1], ids[2], ids[3])
		require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	}
	for _, ids := range [][3]uuid.UUID{{uuid.Nil, org, user}, {event, uuid.Nil, user}, {event, org, uuid.Nil}} {
		_, err := service.NewSessionService(&sessionRepoStub{}).GetSessions(context.Background(), ids[0], ids[1], ids[2], nil, nil)
		require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	}
	for _, args := range [][2]*int{{sessionNumber(0), nil}, {sessionNumber(-1), nil}, {sessionNumber(101), nil}, {nil, sessionNumber(-1)}} {
		_, err := service.NewSessionService(&sessionRepoStub{}).GetSessions(context.Background(), event, org, user, args[0], args[1])
		require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	}
	for _, tc := range []struct {
		limit, offset         *int
		wantLimit, wantOffset int
	}{{nil, nil, 20, 0}, {sessionNumber(100), sessionNumber(5), 100, 5}} {
		calls := 0
		repo := &sessionRepoStub{list: func(ctx context.Context, e, o, u uuid.UUID, limit, offset int) ([]domain.Session, error) {
			calls++
			require.Equal(t, event, e)
			require.Equal(t, org, o)
			require.Equal(t, user, u)
			require.Equal(t, tc.wantLimit, limit)
			require.Equal(t, tc.wantOffset, offset)
			return []domain.Session{{ID: id, EventID: event}}, nil
		}}
		page, err := service.NewSessionService(repo).GetSessions(context.Background(), event, org, user, tc.limit, tc.offset)
		require.NoError(t, err)
		require.Equal(t, 1, calls)
		require.Equal(t, tc.wantLimit, page.Limit)
		require.Equal(t, tc.wantOffset, page.Offset)
		require.Len(t, page.Sessions, 1)
	}
}

func TestSessionReadErrorsPreserved(t *testing.T) {
	event, id, org, user := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, expected := range []error{coreerrors.ErrNotFound, fmt.Errorf("database unavailable")} {
		repo := &sessionRepoStub{
			get: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Session, error) {
				return domain.Session{}, expected
			},
			list: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int, int) ([]domain.Session, error) {
				return nil, expected
			},
		}
		_, err := service.NewSessionService(repo).GetSession(context.Background(), event, id, org, user)
		require.ErrorIs(t, err, expected)
		_, err = service.NewSessionService(repo).GetSessions(context.Background(), event, org, user, nil, nil)
		require.ErrorIs(t, err, expected)
	}
	result := domain.Session{ID: id, EventID: event}
	repo := &sessionRepoStub{get: func(ctx context.Context, e, s, o, u uuid.UUID) (domain.Session, error) {
		require.Equal(t, event, e)
		require.Equal(t, id, s)
		require.Equal(t, org, o)
		require.Equal(t, user, u)
		return result, nil
	}}
	read, err := service.NewSessionService(repo).GetSession(context.Background(), event, id, org, user)
	require.NoError(t, err)
	require.Equal(t, result, read)
}
