package ticket_type_repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *TicketTypeRepo) CreateTicketType(
	ctx context.Context,
	ticketType domain.TicketType,
	eventID, organizerID, userID uuid.UUID,
) (domain.TicketType, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()

	tx, err := r.pool.Pool.Begin(ctx)
	if err != nil {
		return domain.TicketType{}, fmt.Errorf("begin ticket type creation: %w", err)
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	}()

	const accessQuery = `
		SELECT e.status, s.status
		FROM sessions AS s
		JOIN events AS e ON e.id = s.event_id
		JOIN organizer_memberships AS m ON m.organizer_id = e.organizer_id
		WHERE s.id = $1 AND s.event_id = $2 AND e.organizer_id = $3
		  AND m.user_id = $4 AND m.role = 'organizer'
		FOR SHARE OF e, s, m
	`
	var eventStatus domain.EventStatus
	var sessionStatus domain.SessionStatus
	err = tx.QueryRow(ctx, accessQuery,
		ticketType.SessionID, eventID, organizerID, userID,
	).Scan(&eventStatus, &sessionStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TicketType{}, coreerrors.ErrNotFound
	}
	if err != nil {
		return domain.TicketType{}, fmt.Errorf("check ticket type access: %w", err)
	}
	if eventStatus != domain.EventDraft || sessionStatus != domain.SessionDraft {
		return domain.TicketType{}, fmt.Errorf("event and session must be draft: %w", coreerrors.ErrConflict)
	}

	const query = `
		INSERT INTO ticket_types (
			id, session_id, name, price, currency, capacity, available, held, sold, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, session_id, name, price, currency,
		          capacity, available, held, sold, created_at
	`
	var saved ticketTypeModel
	err = tx.QueryRow(ctx, query,
		ticketType.ID,
		ticketType.SessionID,
		ticketType.Name,
		ticketType.Price.Amount,
		ticketType.Price.Currency,
		ticketType.Capacity,
		ticketType.Available,
		ticketType.Held,
		ticketType.Sold,
		ticketType.CreatedAt,
	).Scan(
		&saved.ID,
		&saved.SessionID,
		&saved.Name,
		&saved.Price,
		&saved.Currency,
		&saved.Capacity,
		&saved.Available,
		&saved.Held,
		&saved.Sold,
		&saved.CreatedAt,
	)
	if err != nil {
		return domain.TicketType{}, fmt.Errorf("insert ticket type: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.TicketType{}, fmt.Errorf("commit ticket type: %w", err)
	}
	return toDomain(saved), nil
}
