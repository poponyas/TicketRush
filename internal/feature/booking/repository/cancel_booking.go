package booking_repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	booking_events "github.com/poponyas/TicketRush/internal/feature/booking/events"
)

func (r *BookingRepository) CancelBooking(ctx context.Context, bookingID, userID uuid.UUID) (domain.Booking, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	tx, err := r.pool.Pool.Begin(ctx)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("begin booking cancellation: %w", err)
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	}()

	// A second cancellation waits here, then reads the first one's committed status.
	const lockQuery = `SELECT id,user_id,session_id,status,expires_at,created_at
  FROM booking WHERE id=$1 AND user_id=$2 FOR UPDATE`
	rows, err := tx.Query(ctx, lockQuery, bookingID, userID)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("lock own booking: %w", err)
	}
	booking, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Booking])
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Booking{}, coreerrors.ErrNotFound
	}
	if err != nil {
		return domain.Booking{}, fmt.Errorf("read booking for cancellation: %w", err)
	}
	if booking.Status == domain.BookingCancelled {
		if err := tx.Commit(ctx); err != nil {
			return domain.Booking{}, fmt.Errorf("finish repeated cancellation: %w", err)
		}
		return booking, nil
	}

	if booking.Status != domain.BookingHeld {
		return domain.Booking{}, fmt.Errorf("only held bookings can be cancelled: %w", coreerrors.ErrConflict)
	}
	var expired bool
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp() >= $1::timestamptz`, booking.ExpiresAt).Scan(&expired); err != nil {
		return domain.Booking{}, fmt.Errorf("check booking expiration: %w", err)
	}
	if expired {
		return domain.Booking{}, fmt.Errorf("booking has expired: %w", coreerrors.ErrConflict)
	}

	if err := releaseHeldTickets(ctx, tx, []uuid.UUID{bookingID}); err != nil {
		return domain.Booking{}, err
	}

	// Recheck the deadline after any tariff-lock waits. Failure rolls back every
	// inventory update above, so an expired cancellation never releases tickets.
	const cancelQuery = `UPDATE booking SET status='cancelled'
  WHERE id=$1 AND user_id=$2 AND status='held' AND expires_at > clock_timestamp()
  RETURNING id,user_id,session_id,status,expires_at,created_at`
	rows, err = tx.Query(ctx, cancelQuery, bookingID, userID)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("cancel booking row: %w", err)
	}
	saved, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Booking])
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Booking{}, fmt.Errorf("booking expired during cancellation: %w", coreerrors.ErrConflict)
	}
	if err != nil {
		return domain.Booking{}, fmt.Errorf("read cancelled booking: %w", err)
	}
	if err := insertBookingLifecycleEvents(ctx, tx, []uuid.UUID{saved.ID},
		booking_events.BookingCancelledType, booking_events.BookingCancelledSchemaVersion); err != nil {
		return domain.Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Booking{}, fmt.Errorf("commit booking cancellation: %w", err)
	}
	return saved, nil
}
