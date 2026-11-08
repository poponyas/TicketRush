package booking_repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	booking_events "github.com/poponyas/TicketRush/internal/feature/booking/events"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

func (r *BookingRepository) CreateBooking(
	ctx context.Context,
	booking domain.Booking,
	eventID, ticketTypeID uuid.UUID,
	quantity int,
) (domain.Booking, error) {
	ctx, span := otel.Tracer("ticketrush/booking").
		Start(ctx, "booking.repository.create")
	defer span.End()
	if quantity < 1 || quantity > 1<<31-1 {
		return domain.Booking{}, fmt.Errorf("quantity must be a positive PostgreSQL integer: %w", coreerrors.ErrInvalidArgument)
	}
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()

	tx, err := r.pool.Pool.Begin(ctx)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("begin booking transaction: %w", err)
	}
	defer func() {
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer rollbackCancel()
		_ = tx.Rollback(rollbackCtx)
	}()

	const eventQuery = `SELECT status FROM events WHERE id=$1 FOR SHARE`
	var eventStatus domain.EventStatus
	if err := tx.QueryRow(ctx, eventQuery, eventID).Scan(&eventStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Booking{}, coreerrors.ErrNotFound
		}
		return domain.Booking{}, fmt.Errorf("lock booking event: %w", err)
	}
	if eventStatus != domain.EventPublished {
		return domain.Booking{}, fmt.Errorf("event is not published: %w", coreerrors.ErrNotFound)
	}

	const sessionQuery = `SELECT status,starts_at,sales_start_at,sales_end_at
  FROM sessions WHERE id=$1 AND event_id=$2 FOR SHARE`
	var sessionStatus domain.SessionStatus
	var startsAt, salesStartAt, salesEndAt time.Time
	if err := tx.QueryRow(ctx, sessionQuery, booking.SessionID, eventID).
		Scan(&sessionStatus, &startsAt, &salesStartAt, &salesEndAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Booking{}, coreerrors.ErrNotFound
		}
		return domain.Booking{}, fmt.Errorf("lock booking session: %w", err)
	}
	if sessionStatus != domain.SessionScheduled {
		return domain.Booking{}, fmt.Errorf("session is not scheduled: %w", coreerrors.ErrNotFound)
	}
	lockCtx, lockSpan := otel.Tracer("ticketrush/booking").Start(ctx, "booking.db.lock_ticket_type")

	const tariffQuery = `SELECT id FROM ticket_types WHERE id=$1 AND session_id=$2 FOR UPDATE`
	var lockedID uuid.UUID
	if err := tx.QueryRow(lockCtx, tariffQuery, ticketTypeID, booking.SessionID).Scan(&lockedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			lockSpan.End()

			return domain.Booking{}, coreerrors.ErrNotFound
		}
		lockSpan.RecordError(err)
		lockSpan.SetStatus(codes.Error, "lock ticket type failed")
		lockSpan.End()

		return domain.Booking{}, fmt.Errorf("lock booking ticket type: %w", err)
	}
	lockSpan.End()

	const reserveQuery = `
  UPDATE ticket_types
  SET available=available-$1, held=held+$1
  WHERE id=$2 AND session_id=$3 AND available >= $1
    AND $4::timestamptz <= clock_timestamp()
    AND clock_timestamp() < $5::timestamptz
    AND clock_timestamp() < $6::timestamptz
    AND clock_timestamp() < $7::timestamptz
  RETURNING price,currency
 `
	var unitPrice domain.Money
	err = tx.QueryRow(ctx, reserveQuery, quantity, ticketTypeID, booking.SessionID,
		salesStartAt, salesEndAt, startsAt, booking.ExpiresAt).Scan(&unitPrice.Amount, &unitPrice.Currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Booking{}, fmt.Errorf("insufficient tickets, sales closed or booking expired: %w", coreerrors.ErrConflict)
	}
	if err != nil {
		return domain.Booking{}, fmt.Errorf("reserve booking inventory: %w", err)
	}

	const bookingQuery = `INSERT INTO booking(id,user_id,session_id,status,expires_at,created_at)
  VALUES($1,$2,$3,$4,$5,$6)
  RETURNING id,user_id,session_id,status,expires_at,created_at`
	var saved domain.Booking
	err = tx.QueryRow(ctx, bookingQuery, booking.ID, booking.UserID, booking.SessionID,
		booking.Status, booking.ExpiresAt, booking.CreatedAt).
		Scan(&saved.ID, &saved.UserID, &saved.SessionID, &saved.Status, &saved.ExpiresAt, &saved.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.Booking{}, fmt.Errorf("booking already exists: %w", coreerrors.ErrConflict)
		}
		return domain.Booking{}, fmt.Errorf("insert booking: %w", err)
	}

	const itemQuery = `INSERT INTO booking_item(id,booking_id,ticket_type_id,quantity,unit_price,currency)
  VALUES($1,$2,$3,$4,$5,$6)`
	if _, err := tx.Exec(ctx, itemQuery, uuid.New(), saved.ID, ticketTypeID, quantity,
		unitPrice.Amount, unitPrice.Currency); err != nil {
		return domain.Booking{}, fmt.Errorf("insert booking item: %w", err)
	}

	bookingCreatedPayload := booking_events.BookingCreatedPayload{
		BookingID:    saved.ID,
		EventID:      eventID,
		SessionID:    saved.SessionID,
		TicketTypeID: ticketTypeID,
		Quantity:     quantity,
	}

	if err := insertBookingOutboxEvent(
		ctx,
		tx,
		bookingCreatedPayload,
		booking_events.BookingCreatedType,
		booking_events.BookingCreatedSchemaVersion,
		saved.CreatedAt,
	); err != nil {
		return domain.Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Booking{}, fmt.Errorf("commit booking transaction: %w", err)
	}
	return saved, nil
}
