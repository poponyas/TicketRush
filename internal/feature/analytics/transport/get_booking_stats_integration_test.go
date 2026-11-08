package analytics_transport_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	"github.com/poponyas/TicketRush/internal/feature/analytics"
	booking_events "github.com/poponyas/TicketRush/internal/feature/booking/events"
	"github.com/poponyas/TicketRush/internal/feature/events/testutil"
	"github.com/stretchr/testify/require"
)

func TestBookingStatsAccessPostgres(t *testing.T) {
	f := testutil.New(t)
	event := f.Event(t, f.OrgID, "Report access")
	otherEvent := f.Event(t, f.OtherOrgID, "Foreign report")
	identity := auth.Identity{UserID: f.UserID, OrganizerID: f.OrgID, Roles: []domain.Role{domain.RoleOrganizer}}
	const period = "from=2026-11-08T00:00:00Z&to=2026-11-09T00:00:00Z"
	analyticsCalls := 0
	stats := statsRepository{get: func(ctx context.Context, eventID uuid.UUID, from, to time.Time) ([]analytics.BookingStatsRow, error) {
		analyticsCalls++
		require.Equal(t, event.ID, eventID)
		return []analytics.BookingStatsRow{{EventType: booking_events.BookingCreatedType, Bookings: 1, Tickets: 2}}, nil
	}}
	rw := requestStats(stats, f.Repo, &identity, event.ID.String(), period)
	require.Equal(t, http.StatusOK, rw.Code, rw.Body.String())
	require.Equal(t, 1, analyticsCalls)

	// Even membership in both organizations must not bypass the selected organizer.
	rw = requestStats(stats, f.Repo, &identity, otherEvent.ID.String(), period)
	require.Equal(t, http.StatusNotFound, rw.Code)
	require.Equal(t, 1, analyticsCalls)

	// Claims/context may be stale: check the live database membership on every request.
	f.Exec(t, `UPDATE organizer_memberships SET role='controller' WHERE organizer_id=$1 AND user_id=$2`, f.OrgID, f.UserID)
	rw = requestStats(stats, f.Repo, &identity, event.ID.String(), period)
	require.Equal(t, http.StatusNotFound, rw.Code)
	require.Equal(t, 1, analyticsCalls)
	f.Exec(t, `DELETE FROM organizer_memberships WHERE organizer_id=$1 AND user_id=$2`, f.OrgID, f.UserID)
	rw = requestStats(stats, f.Repo, &identity, event.ID.String(), period)
	require.Equal(t, http.StatusNotFound, rw.Code)
	require.Equal(t, 1, analyticsCalls)
}
