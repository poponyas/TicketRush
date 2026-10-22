package booking_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *BookingService) CancelBooking(ctx context.Context, bookingID, userID uuid.UUID) (domain.Booking, error) {
	if bookingID == uuid.Nil || userID == uuid.Nil {
		return domain.Booking{}, fmt.Errorf("booking and user IDs are required: %w", coreerrors.ErrInvalidArgument)
	}
	booking, err := s.bookingRepo.CancelBooking(ctx, bookingID, userID)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("cancel own booking: %w", err)
	}
	return booking, nil
}
