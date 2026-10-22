package booking_service

import (
	"context"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
)

type BookingService struct {
	bookingRepo BookingRepository
}

type BookingRepository interface {
	ExpireBookings(ctx context.Context, limit int) (int, error)
	CancelBooking(ctx context.Context, bookingID, userID uuid.UUID) (domain.Booking, error)
	GetBooking(ctx context.Context, bookingID, userID uuid.UUID) (domain.Booking, []domain.BookingItem, error)
	// CreateBooking reserves inventory and inserts the booking and its priced item
	// in one transaction. The unit price is read from the tariff inside that transaction.
	CreateBooking(ctx context.Context, booking domain.Booking, eventID, ticketTypeID uuid.UUID, quantity int) (domain.Booking, error)
}

func NewBookingService(bookingRepo BookingRepository) *BookingService {
	return &BookingService{bookingRepo: bookingRepo}
}
