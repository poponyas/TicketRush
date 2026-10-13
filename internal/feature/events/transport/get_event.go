package events_transport

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	core_logger "github.com/poponyas/TicketRush/internal/core/logger"
	core_response "github.com/poponyas/TicketRush/internal/core/response"
	core_transport "github.com/poponyas/TicketRush/internal/core/transport"
)

func (h *EventsHTTPTransport) GetEvent(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := core_logger.FromContext(ctx)

	responseHandler := core_response.NewHTTPResponseHandler(logger, rw)

	identity, ok := auth.FromContext(ctx)
	if !ok {
		responseHandler.ErrorResponse(coreerrors.ErrUnauthenticated, "authentication required")
		return
	}

	if !identity.HasRole(domain.RoleOrganizer) || identity.OrganizerID == uuid.Nil {
		responseHandler.ErrorResponse(coreerrors.ErrForbidden, "organizer access required")
		return
	}

	id, err := core_transport.GetIDInPath(r, "event_id")
	if err != nil {
		responseHandler.ErrorResponse(err, "get id error")
		return
	}

	response, err := h.eventService.GetEvent(ctx, id, identity.OrganizerID, identity.UserID)
	if err != nil {
		responseHandler.ErrorResponse(err, "get event from Transport")
		return
	}

	responseHandler.JSONResponse(eventResponse(response), http.StatusOK)
}
