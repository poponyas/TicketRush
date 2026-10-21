package session_service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/session/service"
	"github.com/stretchr/testify/require"
)

func TestPublicSessionService(t *testing.T) {
	event, session := uuid.New(), uuid.New()
	expected := domain.Session{ID: session, EventID: event, Status: domain.SessionScheduled}
	repo := &sessionRepoStub{publicGet: func(ctx context.Context, e, s uuid.UUID) (domain.Session, error) {
		require.Equal(t, event, e)
		require.Equal(t, session, s)
		return expected, nil
	}}
	got, err := service.NewSessionService(repo).GetPublicSession(context.Background(), event, session)
	require.NoError(t, err)
	require.Equal(t, expected, got)
	for _, ids := range [][2]uuid.UUID{{uuid.Nil, session}, {event, uuid.Nil}} {
		_, err := service.NewSessionService(&sessionRepoStub{}).GetPublicSession(context.Background(), ids[0], ids[1])
		require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	}
}

func TestPublicSessionsPagination(t *testing.T) {
	event := uuid.New()
	for _, tc := range []struct {
		name                  string
		limit, offset         *int
		wantLimit, wantOffset int
	}{
		{"defaults", nil, nil, 20, 0}, {"min", number(1), number(0), 1, 0}, {"max", number(100), number(8), 100, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			repo := &sessionRepoStub{publicList: func(ctx context.Context, e uuid.UUID, l, o int) ([]domain.Session, error) {
				calls++
				require.Equal(t, event, e)
				require.Equal(t, tc.wantLimit, l)
				require.Equal(t, tc.wantOffset, o)
				return []domain.Session{{ID: uuid.New()}}, nil
			}}
			page, err := service.NewSessionService(repo).GetPublicSessions(context.Background(), event, tc.limit, tc.offset)
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			require.Equal(t, tc.wantLimit, page.Limit)
			require.Equal(t, tc.wantOffset, page.Offset)
			require.Len(t, page.Sessions, 1)
		})
	}
	for _, tc := range []struct {
		event         uuid.UUID
		limit, offset *int
	}{
		{uuid.Nil, nil, nil}, {event, number(0), nil}, {event, number(-1), nil}, {event, number(101), nil}, {event, nil, number(-1)},
	} {
		_, err := service.NewSessionService(&sessionRepoStub{}).GetPublicSessions(context.Background(), tc.event, tc.limit, tc.offset)
		require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	}
}
func number(n int) *int { return &n }

func TestPublicSessionsPreserveRepositoryErrors(t *testing.T) {
	for _, want := range []error{coreerrors.ErrNotFound, errors.New("database unavailable")} {
		repo := &sessionRepoStub{
			publicGet: func(context.Context, uuid.UUID, uuid.UUID) (domain.Session, error) {
				return domain.Session{}, fmt.Errorf("query: %w", want)
			},
			publicList: func(context.Context, uuid.UUID, int, int) ([]domain.Session, error) {
				return nil, fmt.Errorf("query: %w", want)
			},
		}
		s := service.NewSessionService(repo)
		_, err := s.GetPublicSession(context.Background(), uuid.New(), uuid.New())
		require.ErrorIs(t, err, want)
		_, err = s.GetPublicSessions(context.Background(), uuid.New(), nil, nil)
		require.ErrorIs(t, err, want)
	}
}
