package booking_repository

import postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"

type BookingRepository struct {
	pool *postgres.Pool
}

func NewBookingRepository(pool *postgres.Pool) *BookingRepository {
	return &BookingRepository{pool: pool}
}
