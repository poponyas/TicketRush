package ticket_type_repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *TicketTypeRepo) GetPublicTicketType(ctx context.Context, eventID, sessionID, ticketTypeID uuid.UUID) (domain.TicketType, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	const query = `SELECT tt.id,tt.session_id,tt.name,tt.price,tt.currency,tt.capacity,tt.available,tt.held,tt.sold,tt.created_at
 FROM ticket_types tt JOIN sessions s ON s.id=tt.session_id JOIN events e ON e.id=s.event_id
 WHERE e.id=$1 AND s.id=$2 AND tt.id=$3 AND e.status='published' AND s.status='scheduled' AND s.starts_at > CURRENT_TIMESTAMP`
	rows, err := r.pool.Pool.Query(ctx, query, eventID, sessionID, ticketTypeID)
	if err != nil {
		return domain.TicketType{}, fmt.Errorf("query public ticket type: %w", err)
	}
	model, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[ticketTypeModel])
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TicketType{}, coreerrors.ErrNotFound
	}
	if err != nil {
		return domain.TicketType{}, fmt.Errorf("collect public ticket type: %w", err)
	}
	return toDomain(model), nil
}

func (r *TicketTypeRepo) GetPublicTicketTypes(ctx context.Context, eventID, sessionID uuid.UUID, limit, offset int) ([]domain.TicketType, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	const parentQuery = `SELECT EXISTS(SELECT 1 FROM sessions s JOIN events e ON e.id=s.event_id
 WHERE e.id=$1 AND s.id=$2 AND e.status='published' AND s.status='scheduled' AND s.starts_at > CURRENT_TIMESTAMP)`
	var visible bool
	if err := r.pool.Pool.QueryRow(ctx, parentQuery, eventID, sessionID).Scan(&visible); err != nil {
		return nil, fmt.Errorf("check public session: %w", err)
	}
	if !visible {
		return nil, coreerrors.ErrNotFound
	}
	const query = `SELECT tt.id,tt.session_id,tt.name,tt.price,tt.currency,tt.capacity,tt.available,tt.held,tt.sold,tt.created_at
 FROM ticket_types tt JOIN sessions s ON s.id=tt.session_id JOIN events e ON e.id=s.event_id
 WHERE e.id=$1 AND s.id=$2 AND e.status='published' AND s.status='scheduled' AND s.starts_at > CURRENT_TIMESTAMP
 ORDER BY tt.created_at ASC,tt.id ASC LIMIT $3 OFFSET $4`
	rows, err := r.pool.Pool.Query(ctx, query, eventID, sessionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query public ticket types: %w", err)
	}
	models, err := pgx.CollectRows(rows, pgx.RowToStructByName[ticketTypeModel])
	if err != nil {
		return nil, fmt.Errorf("collect public ticket types: %w", err)
	}
	return modelsToDomain(models), nil
}
