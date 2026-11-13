// Package testutil provides isolated fixtures for tests against a disposable database.
package testutil

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/poponyas/TicketRush/internal/core/domain"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	repository "github.com/poponyas/TicketRush/internal/feature/events/repository"
	"github.com/stretchr/testify/require"
)

type Fixture struct {
	Pool                                   *pgxpool.Pool
	Repo                                   *repository.EventsRepository
	UserID, OtherUserID, OrgID, OtherOrgID uuid.UUID
	venueIDs                               []uuid.UUID
}

// New only deletes records belonging to UUIDs allocated by this fixture.
func New(t *testing.T) *Fixture {
	t.Helper()
	url := os.Getenv("TICKETRUSH_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TICKETRUSH_TEST_DATABASE_URL to a disposable database with migrations")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx))
	f := &Fixture{Pool: pool, Repo: repository.NewEventsRepo(&postgres.Pool{Pool: pool, OpTimeout: 5 * time.Second}),
		UserID: uuid.New(), OtherUserID: uuid.New(), OrgID: uuid.New(), OtherOrgID: uuid.New()}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		steps := []struct {
			query string
			args  []any
		}{
			{`DELETE FROM tickets WHERE booking_item_id IN (SELECT i.id FROM booking_item i JOIN booking b ON b.id=i.booking_id JOIN sessions s ON s.id=b.session_id JOIN events e ON e.id=s.event_id WHERE e.organizer_id IN ($1,$2))`, []any{f.OrgID, f.OtherOrgID}},
			{`DELETE FROM outbox_events WHERE aggregate_id IN (SELECT b.id FROM booking b JOIN sessions s ON s.id=b.session_id JOIN events e ON e.id=s.event_id WHERE e.organizer_id IN ($1,$2))`, []any{f.OrgID, f.OtherOrgID}},
			{`DELETE FROM booking WHERE session_id IN (SELECT s.id FROM sessions s JOIN events e ON e.id=s.event_id WHERE e.organizer_id IN ($1,$2))`, []any{f.OrgID, f.OtherOrgID}},
			{`DELETE FROM ticket_types WHERE session_id IN (SELECT s.id FROM sessions s JOIN events e ON e.id=s.event_id WHERE e.organizer_id IN ($1,$2))`, []any{f.OrgID, f.OtherOrgID}},
			{`DELETE FROM sessions WHERE event_id IN (SELECT id FROM events WHERE organizer_id IN ($1,$2))`, []any{f.OrgID, f.OtherOrgID}},
			{`DELETE FROM events WHERE organizer_id IN ($1,$2)`, []any{f.OrgID, f.OtherOrgID}},
			{`DELETE FROM venues WHERE id=ANY($1::uuid[])`, []any{f.venueIDs}},
			{`DELETE FROM organizer_memberships WHERE organizer_id IN ($1,$2)`, []any{f.OrgID, f.OtherOrgID}},
			{`DELETE FROM organizers WHERE id IN ($1,$2)`, []any{f.OrgID, f.OtherOrgID}},
			{`DELETE FROM users WHERE id IN ($1,$2)`, []any{f.UserID, f.OtherUserID}},
		}
		for _, step := range steps {
			if _, err := pool.Exec(ctx, step.query, step.args...); err != nil {
				t.Errorf("fixture cleanup: %v", err)
			}
		}
	})
	// These fixtures exercise catalog access, not password login.
	f.Exec(t, `INSERT INTO users(id,email,password_hash) VALUES($1,$2,'unused-test-hash'),($3,$4,'unused-test-hash')`, f.UserID, f.UserID.String()+"@test.invalid", f.OtherUserID, f.OtherUserID.String()+"@test.invalid")
	f.Exec(t, `INSERT INTO organizers(id,name) VALUES($1,'Test organizer'),($2,'Other organizer')`, f.OrgID, f.OtherOrgID)
	// A user can operate two organizations: all queries must also scope organizer_id.
	f.Exec(t, `INSERT INTO organizer_memberships(organizer_id,user_id,role) VALUES($1,$3,'organizer'),($2,$3,'organizer')`, f.OrgID, f.OtherOrgID, f.UserID)
	return f
}

func (f *Fixture) Exec(t *testing.T, query string, args ...any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := f.Pool.Exec(ctx, query, args...)
	require.NoError(t, err)
}

func (f *Fixture) Event(t *testing.T, orgID uuid.UUID, title string) domain.Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	e := domain.NewEvent(orgID, title, "Original description", "conference")
	err := f.Pool.QueryRow(ctx, `INSERT INTO events(id,organizer_id,name,description,category)
		VALUES($1,$2,$3,$4,$5) RETURNING created_at,updated_at,status`, e.ID, e.OrganizerID, e.Title, e.Description, e.Category).
		Scan(&e.CreatedAt, &e.UpdatedAt, &e.Status)
	require.NoError(t, err)
	return e
}

func (f *Fixture) Session(t *testing.T, eventID uuid.UUID) uuid.UUID {
	t.Helper()
	venueID, sessionID := uuid.New(), uuid.New()
	f.venueIDs = append(f.venueIDs, venueID)
	f.Exec(t, `INSERT INTO venues(id,name,city,address) VALUES($1,'Hall','Moscow','Address')`, venueID)
	f.Exec(t, `INSERT INTO sessions(id,event_id,venue_id,starts_at,sales_start_at,sales_end_at)
		VALUES($1,$2,$3,'2027-01-10 18:00+00','2026-01-01 00:00+00','2027-01-10 17:00+00')`, sessionID, eventID, venueID)
	return sessionID
}
