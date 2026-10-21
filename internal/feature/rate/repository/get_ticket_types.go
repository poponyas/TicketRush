package ticket_type_repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *TicketTypeRepo) GetTicketTypes(
	ctx context.Context,
	limit, offset int,
	eventID, sessionID, organizerID, userID uuid.UUID,
) ([]domain.TicketType, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	// Distinguish an accessible session with no tariffs from a missing/foreign session.
	const accessQuery = `SELECT EXISTS(
  SELECT 1 FROM sessions AS s
  JOIN events AS e ON e.id=s.event_id
  JOIN organizer_memberships AS m ON m.organizer_id=e.organizer_id
  WHERE s.id=$1 AND s.event_id=$2 AND e.organizer_id=$3
    AND m.user_id=$4 AND m.role='organizer'
 )`
	var allowed bool
	if err := r.pool.Pool.QueryRow(ctx, accessQuery, sessionID, eventID, organizerID, userID).Scan(&allowed); err != nil {
		return nil, fmt.Errorf("check ticket types access: %w", err)
	}
	if !allowed {
		return nil, coreerrors.ErrNotFound
	}

	const query = `SELECT tt.id,tt.session_id,tt.name,tt.price,tt.currency,
  tt.capacity,tt.available,tt.held,tt.sold,tt.created_at
 FROM ticket_types AS tt
 JOIN sessions AS s ON s.id=tt.session_id
 JOIN events AS e ON e.id=s.event_id
 JOIN organizer_memberships AS m ON m.organizer_id=e.organizer_id
 WHERE tt.session_id=$1 AND s.event_id=$2 AND e.organizer_id=$3
   AND m.user_id=$4 AND m.role='organizer'
 ORDER BY tt.created_at ASC,tt.id ASC
 LIMIT $5 OFFSET $6`
	rows, err := r.pool.Pool.Query(ctx, query, sessionID, eventID, organizerID, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query ticket types: %w", err)
	}
	models, err := pgx.CollectRows(rows, pgx.RowToStructByName[ticketTypeModel])
	if err != nil {
		return nil, fmt.Errorf("collect ticket types: %w", err)
	}
	return modelsToDomain(models), nil
}
