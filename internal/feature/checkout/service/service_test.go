package checkout_service_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/checkout/service"
	"github.com/stretchr/testify/require"
	"testing"
)

type repo struct {
	calls int
	err   error
}

func (r *repo) ConfirmBooking(_ context.Context, b, u uuid.UUID) (domain.Booking, error) {
	r.calls++
	return domain.Booking{ID: b, UserID: u, Status: domain.BookingConfirmed}, r.err
}
func TestCheckoutGateAndErrors(t *testing.T) {
	r := &repo{}
	id, user := uuid.New(), uuid.New()
	_, err := service.New(r, false).Confirm(context.Background(), id, user)
	require.ErrorIs(t, err, coreerrors.ErrForbidden)
	require.Zero(t, r.calls)
	_, err = service.New(r, true).Confirm(context.Background(), uuid.Nil, user)
	require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	require.Zero(t, r.calls)
	b, err := service.New(r, true).Confirm(context.Background(), id, user)
	require.NoError(t, err)
	require.Equal(t, id, b.ID)
	require.Equal(t, user, b.UserID)
	r.err = errors.New("database unavailable")
	_, err = service.New(r, true).Confirm(context.Background(), id, user)
	require.ErrorIs(t, err, r.err)
}
