package booking_service

import (
	"context"
	"fmt"

	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *BookingService) ExpireBookings(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 1000 {
		return 0, fmt.Errorf("expiration batch size must be between 1 and 1000: %w", coreerrors.ErrInvalidArgument)
	}
	count, err := s.bookingRepo.ExpireBookings(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("expire bookings: %w", err)
	}
	return count, nil
}
