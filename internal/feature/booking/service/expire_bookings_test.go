package booking_service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/booking/service"
	"github.com/stretchr/testify/require"
)

type expirationRepo struct {
	service.BookingRepository
	expire func(context.Context, int) (int, error)
}

func (r *expirationRepo) ExpireBookings(ctx context.Context, limit int) (int, error) {
	return r.expire(ctx, limit)
}

func TestExpireBookingsService(t *testing.T) {
	for _, limit := range []int{0, -1, 1001} {
		_, err := service.NewBookingService(nil).ExpireBookings(context.Background(), limit)
		require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	}
	repo := &expirationRepo{expire: func(ctx context.Context, limit int) (int, error) { require.Equal(t, 100, limit); return 37, nil }}
	count, err := service.NewBookingService(repo).ExpireBookings(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 37, count)
	expected := errors.New("database unavailable")
	repo.expire = func(context.Context, int) (int, error) { return 0, fmt.Errorf("query: %w", expected) }
	count, err = service.NewBookingService(repo).ExpireBookings(context.Background(), 100)
	require.ErrorIs(t, err, expected)
	require.Zero(t, count)
}
