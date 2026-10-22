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

type bookingRepoStub struct {
	service.BookingRepository
	get func(context.Context, uuid.UUID, uuid.UUID) (domain.Booking, []domain.BookingItem, error)
}

func (r *bookingRepoStub) GetBooking(ctx context.Context, id, user uuid.UUID) (domain.Booking, []domain.BookingItem, error) {
	return r.get(ctx, id, user)
}
func TestGetBookingService(t *testing.T) {
	id, user := uuid.New(), uuid.New()
	expected := domain.Booking{ID: id, UserID: user, Status: domain.BookingExpired}
	items := []domain.BookingItem{{ID: uuid.New(), BookingID: id, Quantity: 2}}
	repo := &bookingRepoStub{get: func(ctx context.Context, b, u uuid.UUID) (domain.Booking, []domain.BookingItem, error) {
		require.Equal(t, id, b)
		require.Equal(t, user, u)
		return expected, items, nil
	}}
	got, gotItems, err := service.NewBookingService(repo).GetBooking(context.Background(), id, user)
	require.NoError(t, err)
	require.Equal(t, expected, got)
	require.Equal(t, items, gotItems)
}
func TestGetBookingServiceInvalidIDs(t *testing.T) {
	for _, ids := range [][2]uuid.UUID{{uuid.Nil, uuid.New()}, {uuid.New(), uuid.Nil}} {
		_, _, err := service.NewBookingService(&bookingRepoStub{}).GetBooking(context.Background(), ids[0], ids[1])
		require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	}
}
func TestGetBookingServicePreservesErrors(t *testing.T) {
	for _, want := range []error{coreerrors.ErrNotFound, errors.New("database unavailable")} {
		repo := &bookingRepoStub{get: func(context.Context, uuid.UUID, uuid.UUID) (domain.Booking, []domain.BookingItem, error) {
			return domain.Booking{}, nil, fmt.Errorf("repository: %w", want)
		}}
		_, _, err := service.NewBookingService(repo).GetBooking(context.Background(), uuid.New(), uuid.New())
		require.ErrorIs(t, err, want)
	}
}
func TestCreateBookingServiceRejectsInvalidInput(t *testing.T) {
	valid := service.CreateBookingInput{EventID: uuid.New(), SessionID: uuid.New(), TicketTypeID: uuid.New(), UserID: uuid.New(), Quantity: 1}
	for _, change := range []func(*service.CreateBookingInput){
		func(i *service.CreateBookingInput) { i.EventID = uuid.Nil },
		func(i *service.CreateBookingInput) { i.SessionID = uuid.Nil },
		func(i *service.CreateBookingInput) { i.TicketTypeID = uuid.Nil },
		func(i *service.CreateBookingInput) { i.UserID = uuid.Nil },
		func(i *service.CreateBookingInput) { i.Quantity = -1 },
		func(i *service.CreateBookingInput) { i.Quantity = 0 },
		func(i *service.CreateBookingInput) { i.Quantity = 11 },
	} {
		input := valid
		change(&input)
		_, err := service.NewBookingService(nil).CreateBooking(context.Background(), input)
		require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	}
}
