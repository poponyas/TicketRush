package booking_service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	"go.opentelemetry.io/otel"
)

const (
	bookingTTL         = 15 * time.Minute
	maxBookingQuantity = 10
)

func (s *BookingService) CreateBooking(ctx context.Context, input CreateBookingInput) (domain.Booking, error) {
	ctx, span := otel.Tracer("ticketrush/booking").
		Start(ctx, "booking.service.create")
	defer span.End()
	if input.EventID == uuid.Nil || input.SessionID == uuid.Nil || input.TicketTypeID == uuid.Nil || input.UserID == uuid.Nil {
		return domain.Booking{}, fmt.Errorf("IDs must be nonzero: %w", coreerrors.ErrInvalidArgument)
	}

	if input.Quantity > maxBookingQuantity || input.Quantity < 1 {
		return domain.Booking{}, fmt.Errorf("quantity must be between 1 and %d: %w", maxBookingQuantity, coreerrors.ErrInvalidArgument)
	}

	booking := domain.NewBooking(input.UserID, input.SessionID)
	result, err := s.bookingRepo.CreateBooking(ctx, booking, input.EventID, input.TicketTypeID, input.Quantity)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("booking service: %w", err)
	}

	return result, nil
}
