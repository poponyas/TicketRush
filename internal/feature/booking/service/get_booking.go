package booking_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *BookingService) GetBooking(ctx context.Context, bookingID, userID uuid.UUID) (domain.Booking, []domain.BookingItem, error) {
	if bookingID == uuid.Nil || userID == uuid.Nil {
		return domain.Booking{}, nil, fmt.Errorf("booking and user IDs are required: %w", coreerrors.ErrInvalidArgument)
	}
	booking, items, err := s.bookingRepo.GetBooking(ctx, bookingID, userID)
	if err != nil {
		return domain.Booking{}, nil, fmt.Errorf("get own booking: %w", err)
	}
	return booking, items, nil
}
