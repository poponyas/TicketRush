package tickets_repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
)

type Repository struct{ pool *postgres.Pool }

func New(pool *postgres.Pool) *Repository { return &Repository{pool: pool} }

const columns = `t.id,b.id AS booking_id,t.booking_item_id,t.number,b.user_id,b.session_id,
 i.ticket_type_id,t.status,t.created_at,t.used_at,t.checked_in_by`
const joins = ` FROM tickets t JOIN booking_item i ON i.id=t.booking_item_id JOIN booking b ON b.id=i.booking_id `

func (r *Repository) Get(ctx context.Context, id, user uuid.UUID) (domain.Ticket, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	rows, err := r.pool.Pool.Query(ctx, `SELECT `+columns+joins+` WHERE t.id=$1 AND b.user_id=$2`, id, user)
	if err != nil {
		return domain.Ticket{}, fmt.Errorf("query own ticket: %w", err)
	}
	result, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Ticket])
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Ticket{}, coreerrors.ErrNotFound
	}
	if err != nil {
		return domain.Ticket{}, fmt.Errorf("read ticket: %w", err)
	}
	return result, nil
}
func (r *Repository) List(ctx context.Context, user uuid.UUID, limit, offset int) ([]domain.Ticket, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	rows, err := r.pool.Pool.Query(ctx, `SELECT `+columns+joins+` WHERE b.user_id=$1 ORDER BY t.created_at DESC,t.id DESC LIMIT $2 OFFSET $3`, user, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query own tickets: %w", err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[domain.Ticket])
	if err != nil {
		return nil, fmt.Errorf("read own tickets: %w", err)
	}
	return items, nil
}

// CheckIn locks membership and ticket until commit. Concurrent scans cannot
// consume the same ticket twice; revocation of access waits for this operation.
func (r *Repository) CheckIn(ctx context.Context, id, session, user uuid.UUID) (domain.Ticket, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()
	tx, err := r.pool.Pool.Begin(ctx)
	if err != nil {
		return domain.Ticket{}, fmt.Errorf("begin check-in: %w", err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	var allowed uuid.UUID
	err = tx.QueryRow(ctx, `SELECT m.user_id FROM organizer_memberships m
		JOIN events e ON e.organizer_id=m.organizer_id JOIN sessions s ON s.event_id=e.id
		WHERE s.id=$1 AND m.user_id=$2 AND m.role IN ('organizer','controller') AND e.status='published' AND s.status='scheduled' FOR SHARE OF m,e,s`, session, user).Scan(&allowed)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Ticket{}, coreerrors.ErrNotFound
	}
	if err != nil {
		return domain.Ticket{}, fmt.Errorf("check controller membership: %w", err)
	}
	rows, err := tx.Query(ctx, `SELECT `+columns+joins+` WHERE t.id=$1 AND b.session_id=$2 AND b.status='confirmed' FOR UPDATE OF t`, id, session)
	if err != nil {
		return domain.Ticket{}, fmt.Errorf("lock ticket: %w", err)
	}
	ticket, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[domain.Ticket])
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Ticket{}, coreerrors.ErrNotFound
	}
	if err != nil {
		return domain.Ticket{}, fmt.Errorf("read check-in ticket: %w", err)
	}
	if ticket.Status != domain.TicketActive {
		return domain.Ticket{}, coreerrors.ErrConflict
	}
	var usedAt time.Time
	err = tx.QueryRow(ctx, `UPDATE tickets SET status='used',used_at=clock_timestamp(),checked_in_by=$2
		WHERE id=$1 AND status='active' RETURNING used_at`, id, user).Scan(&usedAt)
	if err != nil {
		return domain.Ticket{}, fmt.Errorf("consume ticket: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Ticket{}, fmt.Errorf("commit check-in: %w", err)
	}
	ticket.Status = domain.TicketUsed
	ticket.UsedAt = &usedAt
	ticket.CheckedInBy = &user
	return ticket, nil
}
