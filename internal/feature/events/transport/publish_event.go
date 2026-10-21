package events_transport

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
	core_transport "github.com/poponyas/TicketRush/internal/core/transport"
)

func (t *EventsHTTPTransport) PublishEvent(rw http.ResponseWriter, r *http.Request) {
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

	id, err := core_transport.GetIDInPath(r, "event_id")
	if err != nil {
		h.ErrorResponse(err, "get id error")
		return
	}

	response, err := t.eventService.PublishEvent(ctx, id, identity.OrganizerID, identity.UserID)
	if err != nil {
		h.ErrorResponse(err, "publish event")
		return
	}

	h.JSONResponse(eventResponse(response), http.StatusOK)
}
