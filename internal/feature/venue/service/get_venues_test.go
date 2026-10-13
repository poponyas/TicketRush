package venue_service_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/venue/service"
	"github.com/stretchr/testify/require"
)

type readRepo struct {
	get  func(context.Context, uuid.UUID) (domain.Venue, error)
	list func(context.Context, int, int) ([]domain.Venue, error)
}

func (*readRepo) CreateVenue(context.Context, domain.Venue) (domain.Venue, error) {
	panic("unexpected CreateVenue")
}
func (r *readRepo) GetVenue(ctx context.Context, id uuid.UUID) (domain.Venue, error) {
	return r.get(ctx, id)
}
func (r *readRepo) GetVenues(ctx context.Context, limit, offset int) ([]domain.Venue, error) {
	return r.list(ctx, limit, offset)
}
func intPointer(n int) *int { return &n }

func TestVenueReadValidation(t *testing.T) {
	_, err := service.NewVenueService(&readRepo{}).GetVenue(context.Background(), uuid.Nil)
	require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	for _, tc := range []struct{ limit, offset *int }{{intPointer(0), nil}, {intPointer(-1), nil}, {intPointer(101), nil}, {nil, intPointer(-1)}} {
		_, err := service.NewVenueService(&readRepo{}).GetVenues(context.Background(), tc.limit, tc.offset)
		require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	}
	for _, tc := range []struct {
		limit, offset         *int
		wantLimit, wantOffset int
	}{{nil, nil, 20, 0}, {intPointer(100), intPointer(2), 100, 2}} {
		calls := 0
		r := &readRepo{list: func(ctx context.Context, limit, offset int) ([]domain.Venue, error) {
			calls++
			require.Equal(t, tc.wantLimit, limit)
			require.Equal(t, tc.wantOffset, offset)
			return []domain.Venue{{ID: uuid.New()}}, nil
		}}
		page, err := service.NewVenueService(r).GetVenues(context.Background(), tc.limit, tc.offset)
		require.NoError(t, err)
		require.Equal(t, 1, calls)
		require.Equal(t, tc.wantLimit, page.Limit)
		require.Equal(t, tc.wantOffset, page.Offset)
		require.Len(t, page.Venues, 1)
	}
}

func TestVenueReadErrorsPreserved(t *testing.T) {
	for _, expected := range []error{coreerrors.ErrNotFound, fmt.Errorf("database unavailable")} {
		r := &readRepo{
			get:  func(context.Context, uuid.UUID) (domain.Venue, error) { return domain.Venue{}, expected },
			list: func(context.Context, int, int) ([]domain.Venue, error) { return nil, expected },
		}
		_, err := service.NewVenueService(r).GetVenue(context.Background(), uuid.New())
		require.ErrorIs(t, err, expected)
		_, err = service.NewVenueService(r).GetVenues(context.Background(), nil, nil)
		require.ErrorIs(t, err, expected)
	}
}
