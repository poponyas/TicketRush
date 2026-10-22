package booking_repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *BookingRepository) GetBooking(ctx context.Context, bookingID, userID uuid.UUID) (domain.Booking, []domain.BookingItem, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	const query = `
  SELECT b.id AS booking_id,b.user_id,b.session_id,b.status,b.expires_at,b.created_at,
         i.id AS item_id,i.ticket_type_id,i.quantity,i.unit_price,i.currency
  FROM booking b
  LEFT JOIN booking_item i ON i.booking_id=b.id
  WHERE b.id=$1 AND b.user_id=$2
  ORDER BY i.id ASC
 `
	rows, err := r.pool.Pool.Query(ctx, query, bookingID, userID)
	if err != nil {
		return domain.Booking{}, nil, fmt.Errorf("query own booking: %w", err)
	}
	models, err := pgx.CollectRows(rows, pgx.RowToStructByName[bookingDetailsRow])
	if err != nil {
		return domain.Booking{}, nil, fmt.Errorf("collect booking details: %w", err)
	}
	if len(models) == 0 {
		return domain.Booking{}, nil, coreerrors.ErrNotFound
	}
	first := models[0]
	booking := domain.Booking{ID: first.BookingID, UserID: first.UserID, SessionID: first.SessionID,
		Status: first.Status, ExpiresAt: first.ExpiresAt, CreatedAt: first.CreatedAt}
	items := make([]domain.BookingItem, 0, len(models))
	for _, row := range models {
		if row.ItemID == nil {
			continue
		}
		if row.TicketTypeID == nil || row.Quantity == nil || row.UnitPrice == nil || row.Currency == nil {
			return domain.Booking{}, nil, fmt.Errorf("incomplete booking item row")
		}
		items = append(items, domain.BookingItem{ID: *row.ItemID, BookingID: row.BookingID,
			TicketTypeID: *row.TicketTypeID, Quantity: *row.Quantity,
			UnitPrice: domain.Money{Amount: *row.UnitPrice, Currency: *row.Currency}})
	}
	return booking, items, nil
}
