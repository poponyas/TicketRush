package events_transport

import (
	"net/http"

	core_logger "github.com/poponyas/TicketRush/internal/core/logger"
	core_response "github.com/poponyas/TicketRush/internal/core/response"
	core_transport "github.com/poponyas/TicketRush/internal/core/transport"
)

func (t *EventsHTTPTransport) GetPublicEvents(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := core_logger.FromContext(ctx)
	responseHandler := core_response.NewHTTPResponseHandler(log, rw)

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

	result, err := t.eventService.GetPublicEvents(ctx, limit, offset)
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
