package booking_transport

import (
	"context"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	service "github.com/poponyas/TicketRush/internal/feature/booking/service"
)

type HTTPBookingTransport struct {
	bookingService BookingService
}

type BookingService interface {
	CancelBooking(ctx context.Context, bookingID, userID uuid.UUID) (domain.Booking, error)
	CreateBooking(ctx context.Context, input service.CreateBookingInput) (domain.Booking, error)
	GetBooking(ctx context.Context, bookingID uuid.UUID, userID uuid.UUID) (domain.Booking, []domain.BookingItem, error)
}

func NewHTTPBookingTransport(bookingService BookingService) *HTTPBookingTransport {
	return &HTTPBookingTransport{bookingService: bookingService}
}
