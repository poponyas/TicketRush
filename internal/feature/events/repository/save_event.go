package events_repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *EventsRepository) CreateEvent(ctx context.Context, event domain.Event, actorID uuid.UUID) (domain.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	// Membership is checked in the same SQL statement as the insertion.
	// Client input cannot grant ownership of another organizer's event.
	const query = `INSERT INTO events (id,organizer_id,name,description,category,status)
        SELECT $1,$2,$3,$4,$5,$6
        FROM organizer_memberships
        WHERE organizer_id=$2 AND user_id=$7 AND role='organizer'
        RETURNING id,organizer_id,name,description,category,poster_key,status,created_at,updated_at`
	var saved domain.Event
	err := r.pool.Pool.QueryRow(ctx, query, event.ID, event.OrganizerID, event.Title,
		event.Description, event.Category, event.Status, actorID).Scan(
		&saved.ID, &saved.OrganizerID, &saved.Title, &saved.Description, &saved.Category,
		&saved.PosterKey, &saved.Status, &saved.CreatedAt, &saved.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Event{}, coreerrors.ErrForbidden
	}
	if err != nil {
		return domain.Event{}, fmt.Errorf("insert event: %w", err)
	}
	return saved, nil
}
