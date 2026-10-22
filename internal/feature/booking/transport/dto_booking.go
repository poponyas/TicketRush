package booking_transport

import (
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
)

type CreateBookingRequest struct {
	TicketTypeID uuid.UUID `json:"ticket_type_id"`
	Quantity     int       `json:"quantity"`
}

type BookingResponse struct {
	ID        uuid.UUID            `json:"id"`
	SessionID uuid.UUID            `json:"session_id"`
	Status    domain.BookingStatus `json:"status"`
	ExpiresAt time.Time            `json:"expires_at"`
}

type BookingGetResponse struct {
	ID        uuid.UUID             `json:"id"`
	SessionID uuid.UUID             `json:"session_id"`
	Status    domain.BookingStatus  `json:"status"`
	ExpiresAt time.Time             `json:"expires_at"`
	Items     []BookingItemResponse `json:"items"`
}

func bookingResponse(booking domain.Booking) BookingResponse {
	return BookingResponse{
		ID:        booking.ID,
		SessionID: booking.SessionID,
		Status:    booking.Status,
		ExpiresAt: booking.ExpiresAt,
	}
}

type BookingItemResponse struct {
	TicketTypeID uuid.UUID    `json:"ticket_type_id"`
	Quantity     int          `json:"quantity"`
	UnitPrice    domain.Money `json:"unit_price"`
}

func bookingGetResponse(booking domain.Booking, items []domain.BookingItem) BookingGetResponse {
	result := BookingGetResponse{ID: booking.ID, SessionID: booking.SessionID, Status: booking.Status,
		ExpiresAt: booking.ExpiresAt, Items: make([]BookingItemResponse, 0, len(items))}
	for _, item := range items {
		result.Items = append(result.Items, BookingItemResponse{TicketTypeID: item.TicketTypeID,
			Quantity: item.Quantity, UnitPrice: item.UnitPrice})
	}
	return result
}
