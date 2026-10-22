package booking_service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/booking/service"
	"github.com/stretchr/testify/require"
)

type cancellationRepo struct {
	service.BookingRepository
	cancel func(context.Context, uuid.UUID, uuid.UUID) (domain.Booking, error)
}

func (r *cancellationRepo) CancelBooking(ctx context.Context, id, user uuid.UUID) (domain.Booking, error) {
	return r.cancel(ctx, id, user)
}

func TestCancelBookingService(t *testing.T) {
	id, user := uuid.New(), uuid.New()
	expected := domain.Booking{ID: id, UserID: user, Status: domain.BookingCancelled}
	repo := &cancellationRepo{cancel: func(ctx context.Context, b, u uuid.UUID) (domain.Booking, error) {
		require.Equal(t, id, b)
		require.Equal(t, user, u)
		return expected, nil
	}}
	result, err := service.NewBookingService(repo).CancelBooking(context.Background(), id, user)
	require.NoError(t, err)
	require.Equal(t, expected, result)
	for _, ids := range [][2]uuid.UUID{{uuid.Nil, user}, {id, uuid.Nil}} {
		_, err := service.NewBookingService(nil).CancelBooking(context.Background(), ids[0], ids[1])
		require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	}
	for _, want := range []error{coreerrors.ErrNotFound, coreerrors.ErrConflict, errors.New("database unavailable")} {
		repo.cancel = func(context.Context, uuid.UUID, uuid.UUID) (domain.Booking, error) {
			return domain.Booking{}, fmt.Errorf("repository: %w", want)
		}
		_, err := service.NewBookingService(repo).CancelBooking(context.Background(), id, user)
		require.ErrorIs(t, err, want)
	}
}
