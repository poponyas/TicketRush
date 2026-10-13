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

func (h *EventsHTTPTransport) GetEvents(rw http.ResponseWriter, r *http.Request) {
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

	limit, err := core_transport.GetQueryByURL(r, "limit")
	if err != nil {
		responseHandler.ErrorResponse(err, "can't get limit from url")
		return
	}

	offset, err := core_transport.GetQueryByURL(r, "offset")
	if err != nil {
		responseHandler.ErrorResponse(err, "can't get offset from url")
		return
	}

	result, err := h.eventService.GetEvents(ctx, limit, offset, identity.OrganizerID, identity.UserID)
	if err != nil {
		responseHandler.ErrorResponse(err, "can't getEvents from Transport")
		return
	}

	responseHandler.JSONResponse(EventsListResponse{
		Items:  eventsResponse(result.Events),
		Limit:  result.Limit,
		Offset: result.Offset,
	}, http.StatusOK)
}
