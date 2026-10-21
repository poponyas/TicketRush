package ticket_type_repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *TicketTypeRepo) GetTicketType(
	ctx context.Context,
	ticketTypeID uuid.UUID,
	eventID uuid.UUID,
	sessionID uuid.UUID,
	organizerID uuid.UUID,
	userID uuid.UUID,
) (domain.TicketType, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()

	const query = `
	SELECT tt.id, tt.session_id, tt.name, tt.price, tt.currency, tt.capacity, tt.available, tt.held, tt.sold, tt.created_at
	FROM ticket_types AS tt
	JOIN sessions AS s ON s.id = tt.session_id
	JOIN events AS e ON e.id = s.event_id
	JOIN organizer_memberships AS m ON m.organizer_id = e.organizer_id
	WHERE tt.id =  $1 AND e.id = $2 AND s.id = $3 AND m.organizer_id = $4 AND m.user_id = $5 AND m.role = 'organizer';
	`

	rows, err := r.pool.Pool.Query(
		ctx, query,
		ticketTypeID, eventID, sessionID, organizerID, userID,
	)
	if err != nil {
		return domain.TicketType{}, fmt.Errorf("query ticket type: %w", err)
	}

	model, err := pgx.CollectExactlyOneRow(
		rows,
		pgx.RowToStructByName[ticketTypeModel],
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.TicketType{}, core_errors.ErrNotFound
		}
		return domain.TicketType{}, fmt.Errorf("collect ticket type: %w", err)
	}

	return toDomain(model), nil
}
