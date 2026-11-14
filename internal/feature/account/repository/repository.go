package account_repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	"github.com/poponyas/TicketRush/internal/feature/account"
)

type Repository struct{ pool *postgres.Pool }

func New(pool *postgres.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Profile(ctx context.Context, userID uuid.UUID) (account.Profile, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	var p account.Profile
	err := r.pool.Pool.QueryRow(ctx, `SELECT u.id,u.email,u.display_name,
	 EXISTS(SELECT 1 FROM user_roles r WHERE r.user_id=u.id AND r.role='ownerVenue')
	 FROM users u WHERE u.id=$1`, userID).Scan(&p.ID, &p.Email, &p.DisplayName, &p.VenueOwner)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, coreerrors.ErrNotFound
	}
	if err != nil {
		return p, fmt.Errorf("account profile: %w", err)
	}
	rows, err := r.pool.Pool.Query(ctx, `SELECT o.id,o.name,m.role FROM organizers o
	 JOIN organizer_memberships m ON m.organizer_id=o.id
	 WHERE m.user_id=$1 ORDER BY o.name,o.id`, userID)
	if err != nil {
		return p, fmt.Errorf("account organizations: %w", err)
	}
	p.Organizations, err = pgx.CollectRows(rows, pgx.RowToStructByName[account.Organization])
	if err != nil {
		return p, fmt.Errorf("collect account organizations: %w", err)
	}
	if p.Organizations == nil {
		p.Organizations = []account.Organization{}
	}
	return p, nil
}

func (r *Repository) Bookings(ctx context.Context, userID uuid.UUID, limit, offset int) ([]account.Booking, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	// Price is the snapshot from booking_item, never the current tariff price.
	rows, err := r.pool.Pool.Query(ctx, `SELECT b.id,e.id AS event_id,b.session_id,
	 e.name AS event_name,v.name AS venue_name,v.city,b.status,s.starts_at,
	 b.expires_at,b.created_at,COALESCE(i.quantity,0) AS quantity,
	 COALESCE(i.total,0)::text AS total,COALESCE(i.currency,'RUB') AS currency
	 FROM booking b JOIN sessions s ON s.id=b.session_id
	 JOIN events e ON e.id=s.event_id JOIN venues v ON v.id=s.venue_id
	 LEFT JOIN LATERAL (SELECT sum(quantity)::bigint AS quantity,
	 sum(unit_price*quantity) AS total,min(currency) AS currency
	 FROM booking_item WHERE booking_id=b.id) i ON true
	 WHERE b.user_id=$1 ORDER BY b.created_at DESC,b.id DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("account bookings: %w", err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[account.Booking])
	if err != nil {
		return nil, fmt.Errorf("collect account bookings: %w", err)
	}
	if items == nil {
		items = []account.Booking{}
	}
	return items, nil
}
