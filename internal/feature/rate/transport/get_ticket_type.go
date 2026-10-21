package ticket_type_transport

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

func (t *HTTPTicketType) GetTicketType(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := core_logger.FromContext(ctx)
	responseHandler := core_response.NewHTTPResponseHandler(logger, rw)

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
		responseHandler.ErrorResponse(err, "can't get eventID")
		return
	}

	sessionID, err := core_transport.GetIDInPath(r, "session_id")
	if err != nil {
		responseHandler.ErrorResponse(err, "can't get sessionID")
		return
	}

	ticketTypeID, err := core_transport.GetIDInPath(r, "ticket_type_id")
	if err != nil {
		responseHandler.ErrorResponse(err, "can't get ticketTypeID")
		return
	}

	response, err := t.ticketTypeService.GetTicketType(ctx, ticketTypeID, eventID, sessionID, identity.OrganizerID, identity.UserID)
	if err != nil {
		responseHandler.ErrorResponse(err, "get ticket type")
		return
	}

	responseHandler.JSONResponse(
		domainToTicketTypeResponse(response),
		http.StatusOK,
	)
}
