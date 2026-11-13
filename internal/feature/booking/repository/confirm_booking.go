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
	bookingevents "github.com/poponyas/TicketRush/internal/feature/booking/events"
)

// ConfirmBooking is the atomic persistence operation behind the mock checkout.
// The booking lock serializes confirmation, cancellation and expiration.
func (r *BookingRepository) ConfirmBooking(ctx context.Context, bookingID, userID uuid.UUID) (domain.Booking, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	tx, err := r.pool.Pool.Begin(ctx)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("begin confirmation: %w", err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	var booking domain.Booking
	err = tx.QueryRow(ctx, `SELECT id,user_id,session_id,status,expires_at,created_at
		FROM booking WHERE id=$1 AND user_id=$2 FOR UPDATE`, bookingID, userID).
		Scan(&booking.ID, &booking.UserID, &booking.SessionID, &booking.Status, &booking.ExpiresAt, &booking.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Booking{}, coreerrors.ErrNotFound
	}
	if err != nil {
		return domain.Booking{}, fmt.Errorf("lock booking for confirmation: %w", err)
	}
	// The first transaction already issued tickets and published an outbox row.
	if booking.Status == domain.BookingConfirmed {
		if err := tx.Commit(ctx); err != nil {
			return domain.Booking{}, fmt.Errorf("repeat confirmation: %w", err)
		}
		return booking, nil
	}
	if booking.Status != domain.BookingHeld {
		return domain.Booking{}, coreerrors.ErrConflict
	}
	var eventID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT event_id FROM sessions WHERE id=$1`, booking.SessionID).Scan(&eventID); err != nil {
		return domain.Booking{}, fmt.Errorf("find confirmation event: %w", err)
	}
	var eventStatus domain.EventStatus
	if err := tx.QueryRow(ctx, `SELECT status FROM events WHERE id=$1 FOR SHARE`, eventID).Scan(&eventStatus); err != nil {
		return domain.Booking{}, fmt.Errorf("lock confirmation event: %w", err)
	}
	var status domain.SessionStatus
	var startsAt time.Time
	if err := tx.QueryRow(ctx, `SELECT status,starts_at FROM sessions WHERE id=$1 AND event_id=$2 FOR SHARE`, booking.SessionID, eventID).Scan(&status, &startsAt); err != nil {
		return domain.Booking{}, fmt.Errorf("lock confirmation session: %w", err)
	}
	if eventStatus != domain.EventPublished || status != domain.SessionScheduled {
		return domain.Booking{}, coreerrors.ErrConflict
	}
	rows, err := tx.Query(ctx, `SELECT ticket_type_id,SUM(quantity)::bigint FROM booking_item
		WHERE booking_id=$1 GROUP BY ticket_type_id ORDER BY ticket_type_id`, bookingID)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("query confirmation positions: %w", err)
	}
	type item struct {
		id       uuid.UUID
		quantity int64
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (item, error) {
		var i item
		err := row.Scan(&i.id, &i.quantity)
		return i, err
	})
	if err != nil {
		return domain.Booking{}, fmt.Errorf("read confirmation positions: %w", err)
	}
	if len(items) == 0 {
		return domain.Booking{}, coreerrors.ErrConflict
	}
	for _, i := range items {
		tag, err := tx.Exec(ctx, `UPDATE ticket_types SET held=held-$1,sold=sold+$1
			WHERE id=$2 AND session_id=$3 AND held >= $1`, i.quantity, i.id, booking.SessionID)
		if err != nil {
			return domain.Booking{}, fmt.Errorf("sell held tickets: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return domain.Booking{}, coreerrors.ErrConflict
		}
	}
	// Recheck time after tariff-lock waits; rollback restores counters on failure.
	tag, err := tx.Exec(ctx, `UPDATE booking SET status='confirmed' WHERE id=$1 AND status='held'
		AND expires_at>clock_timestamp() AND $2::timestamptz>clock_timestamp()`, bookingID, startsAt)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("confirm booking: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return domain.Booking{}, coreerrors.ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO tickets(id,booking_item_id,number)
		SELECT gen_random_uuid(),i.id,n FROM booking_item i CROSS JOIN LATERAL generate_series(1,i.quantity) n
		WHERE i.booking_id=$1`, bookingID)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("issue tickets: %w", err)
	}
	if err := insertBookingLifecycleEvents(ctx, tx, []uuid.UUID{bookingID}, bookingevents.BookingConfirmedType, bookingevents.BookingConfirmedSchemaVersion); err != nil {
		return domain.Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Booking{}, fmt.Errorf("commit confirmation: %w", err)
	}
	booking.Status = domain.BookingConfirmed
	return booking, nil
}
