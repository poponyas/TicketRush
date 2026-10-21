package ticket_type_service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/rate/service"
	"github.com/stretchr/testify/require"
)

type publicRepo struct {
	service.TicketTypeRepo
	get  func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.TicketType, error)
	list func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]domain.TicketType, error)
}

func (r *publicRepo) GetPublicTicketType(ctx context.Context, e, s, tt uuid.UUID) (domain.TicketType, error) {
	return r.get(ctx, e, s, tt)
}
func (r *publicRepo) GetPublicTicketTypes(ctx context.Context, e, s uuid.UUID, l, o int) ([]domain.TicketType, error) {
	return r.list(ctx, e, s, l, o)
}
func integer(n int) *int { return &n }

func TestPublicTicketTypeService(t *testing.T) {
	event, session, id := uuid.New(), uuid.New(), uuid.New()
	expected := domain.TicketType{ID: id, SessionID: session, Name: "Standard"}
	repo := &publicRepo{get: func(ctx context.Context, e, s, tt uuid.UUID) (domain.TicketType, error) {
		require.Equal(t, event, e)
		require.Equal(t, session, s)
		require.Equal(t, id, tt)
		return expected, nil
	}}
	got, err := service.NewTicketTypeService(repo).GetPublicTicketType(context.Background(), event, session, id)
	require.NoError(t, err)
	require.Equal(t, expected, got)
	for _, ids := range [][3]uuid.UUID{{uuid.Nil, session, id}, {event, uuid.Nil, id}, {event, session, uuid.Nil}} {
		_, err := service.NewTicketTypeService(&publicRepo{}).GetPublicTicketType(context.Background(), ids[0], ids[1], ids[2])
		require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	}
}
func TestPublicTicketTypesPagination(t *testing.T) {
	event, session := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name                  string
		limit, offset         *int
		wantLimit, wantOffset int
	}{
		{"defaults", nil, nil, 20, 0}, {"min", integer(1), integer(0), 1, 0}, {"max", integer(100), integer(9), 100, 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			repo := &publicRepo{list: func(ctx context.Context, e, s uuid.UUID, l, o int) ([]domain.TicketType, error) {
				calls++
				require.Equal(t, event, e)
				require.Equal(t, session, s)
				require.Equal(t, tc.wantLimit, l)
				require.Equal(t, tc.wantOffset, o)
				return []domain.TicketType{{ID: uuid.New()}}, nil
			}}
			page, err := service.NewTicketTypeService(repo).GetPublicTicketTypes(context.Background(), event, session, tc.limit, tc.offset)
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			require.Equal(t, tc.wantLimit, page.Limit)
			require.Equal(t, tc.wantOffset, page.Offset)
			require.Len(t, page.TicketTypes, 1)
		})
	}
	for _, tc := range []struct {
		e, s uuid.UUID
		l, o *int
	}{
		{uuid.Nil, session, nil, nil}, {event, uuid.Nil, nil, nil}, {event, session, integer(0), nil},
		{event, session, integer(-1), nil}, {event, session, integer(101), nil}, {event, session, nil, integer(-1)},
	} {
		_, err := service.NewTicketTypeService(&publicRepo{}).GetPublicTicketTypes(context.Background(), tc.e, tc.s, tc.l, tc.o)
		require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	}
}
func TestPublicTicketTypesPreserveErrors(t *testing.T) {
	for _, want := range []error{coreerrors.ErrNotFound, errors.New("database unavailable")} {
		repo := &publicRepo{
			get: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.TicketType, error) {
				return domain.TicketType{}, fmt.Errorf("query: %w", want)
			},
			list: func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]domain.TicketType, error) {
				return nil, fmt.Errorf("query: %w", want)
			},
		}
		s := service.NewTicketTypeService(repo)
		_, err := s.GetPublicTicketType(context.Background(), uuid.New(), uuid.New(), uuid.New())
		require.ErrorIs(t, err, want)
		_, err = s.GetPublicTicketTypes(context.Background(), uuid.New(), uuid.New(), nil, nil)
		require.ErrorIs(t, err, want)
	}
}
