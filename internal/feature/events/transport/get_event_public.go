package events_transport

import (
	"net/http"

	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
	coretransport "github.com/poponyas/TicketRush/internal/core/transport"
)

func (t *EventsHTTPTransport) GetPublicEvent(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h := response.NewHTTPResponseHandler(corelogger.FromContext(ctx), rw)
	eventID, err := coretransport.GetIDInPath(r, "event_id")
	if err != nil {
		h.ErrorResponse(err, "get public event ID")
		return
	}
	event, err := t.eventService.GetPublicEvent(ctx, eventID)
	if err != nil {
		h.ErrorResponse(err, "get public event")
		return
	}
	h.JSONResponse(eventResponse(event), http.StatusOK)
}
