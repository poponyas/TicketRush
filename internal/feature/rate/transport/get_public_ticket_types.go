package ticket_type_transport

import (
	"net/http"

	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
	coretransport "github.com/poponyas/TicketRush/internal/core/transport"
)

func (t *HTTPTicketType) GetPublicTicketType(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h := response.NewHTTPResponseHandler(corelogger.FromContext(ctx), rw)
	eventID, err := coretransport.GetIDInPath(r, "event_id")
	if err != nil {
		h.ErrorResponse(err, "get event_id")
		return
	}
	sessionID, err := coretransport.GetIDInPath(r, "session_id")
	if err != nil {
		h.ErrorResponse(err, "get session_id")
		return
	}
	ticketTypeID, err := coretransport.GetIDInPath(r, "ticket_type_id")
	if err != nil {
		h.ErrorResponse(err, "get ticket_type_id")
		return
	}
	result, err := t.ticketTypeService.GetPublicTicketType(ctx, eventID, sessionID, ticketTypeID)
	if err != nil {
		h.ErrorResponse(err, "get public tickettype")
		return
	}
	h.JSONResponse(domainToTicketTypeResponse(result), http.StatusOK)
}

func (t *HTTPTicketType) GetPublicTicketTypes(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h := response.NewHTTPResponseHandler(corelogger.FromContext(ctx), rw)
	eventID, err := coretransport.GetIDInPath(r, "event_id")
	if err != nil {
		h.ErrorResponse(err, "get event_id")
		return
	}
	sessionID, err := coretransport.GetIDInPath(r, "session_id")
	if err != nil {
		h.ErrorResponse(err, "get session_id")
		return
	}
	limit, err := coretransport.GetQueryByURL(r, "limit")
	if err != nil {
		h.ErrorResponse(err, "get limit")
		return
	}
	offset, err := coretransport.GetQueryByURL(r, "offset")
	if err != nil {
		h.ErrorResponse(err, "get offset")
		return
	}
	result, err := t.ticketTypeService.GetPublicTicketTypes(ctx, eventID, sessionID, limit, offset)
	if err != nil {
		h.ErrorResponse(err, "get public tickettypes")
		return
	}
	h.JSONResponse(TicketTypesListResponse{Items: domainToTicketTypesResponse(result.TicketTypes), Limit: result.Limit, Offset: result.Offset}, http.StatusOK)
}
