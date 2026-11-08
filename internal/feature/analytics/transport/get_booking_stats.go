package analytics_transport

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
	coretransport "github.com/poponyas/TicketRush/internal/core/transport"
	service "github.com/poponyas/TicketRush/internal/feature/analytics/service"
)

func (t *HTTPAnalyticsTransport) GetBookingStats(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h := response.NewHTTPResponseHandler(corelogger.FromContext(ctx), rw)
	identity, ok := auth.FromContext(ctx)
	if !ok {
		h.ErrorResponse(coreerrors.ErrUnauthenticated, "authentication required")
		return
	}
	if !identity.HasRole(domain.RoleOrganizer) || identity.OrganizerID == uuid.Nil {
		h.ErrorResponse(coreerrors.ErrForbidden, "organizer access required")
		return
	}
	eventID, err := coretransport.GetIDInPath(r, "event_id")
	if err != nil {
		h.ErrorResponse(err, "get event ID for booking stats")
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		h.ErrorResponse(coreerrors.ErrInvalidArgument, "invalid booking stats query")
		return
	}
	from, err := queryTime(query, "from")
	if err != nil {
		h.ErrorResponse(err, "parse booking stats from")
		return
	}
	to, err := queryTime(query, "to")
	if err != nil {
		h.ErrorResponse(err, "parse booking stats to")
		return
	}
	report, err := t.analyticsService.GetBookingStats(ctx, service.BookingStatsInput{
		EventID: eventID, OrganizerID: identity.OrganizerID, UserID: identity.UserID,
		From: from, To: to,
	})
	if err != nil {
		h.ErrorResponse(err, "get booking stats")
		return
	}
	h.JSONResponse(bookingStatsResponse(report), http.StatusOK)
}

func queryTime(query url.Values, key string) (time.Time, error) {
	values := query[key]
	if len(values) != 1 || values[0] == "" {
		return time.Time{}, fmt.Errorf("%w: one %s timestamp is required", coreerrors.ErrInvalidArgument, key)
	}
	value, err := time.Parse(time.RFC3339Nano, values[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s must be RFC3339: %v", coreerrors.ErrInvalidArgument, key, err)
	}
	return value.UTC(), nil
}
