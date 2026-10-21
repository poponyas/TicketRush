package session_transport

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
	core_logger "github.com/poponyas/TicketRush/internal/core/logger"
	core_response "github.com/poponyas/TicketRush/internal/core/response"
	core_transport "github.com/poponyas/TicketRush/internal/core/transport"
)

func (t *HTTPSessionTransport) PublishSession(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := core_logger.FromContext(ctx)
	responseHandler := core_response.NewHTTPResponseHandler(log, rw)
	identity, ok := auth.FromContext(ctx)
	if !ok {
		responseHandler.ErrorResponse(core_errors.ErrUnauthenticated, "authentication required")
		return
	}
	if !identity.HasRole(domain.RoleOrganizer) || identity.OrganizerID == uuid.Nil {
		responseHandler.ErrorResponse(core_errors.ErrForbidden, "organizer access required")
		return
	}

	eventID, err := core_transport.GetIDInPath(r, "event_id")
	if err != nil {
		responseHandler.ErrorResponse(err, "get event ID")
		return
	}
	sessionID, err := core_transport.GetIDInPath(r, "session_id")
	if err != nil {
		responseHandler.ErrorResponse(err, "get session ID")
		return
	}

	response, err := t.sessionService.PublishSession(ctx, eventID, sessionID, identity.OrganizerID, identity.UserID)
	if err != nil {
		responseHandler.ErrorResponse(err, "publish session")
		return
	}

	responseHandler.JSONResponse(domainSessionToDTO(response), http.StatusOK)
}
