package session_transport

import (
	"net/http"

	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
	coretransport "github.com/poponyas/TicketRush/internal/core/transport"
)

func (t *HTTPSessionTransport) GetPublicSession(rw http.ResponseWriter, r *http.Request) {
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
	result, err := t.sessionService.GetPublicSession(ctx, eventID, sessionID)
	if err != nil {
		h.ErrorResponse(err, "get public session")
		return
	}
	h.JSONResponse(domainSessionToDTO(result), http.StatusOK)
}

func (t *HTTPSessionTransport) GetPublicSessions(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h := response.NewHTTPResponseHandler(corelogger.FromContext(ctx), rw)
	eventID, err := coretransport.GetIDInPath(r, "event_id")
	if err != nil {
		h.ErrorResponse(err, "get event_id")
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
	result, err := t.sessionService.GetPublicSessions(ctx, eventID, limit, offset)
	if err != nil {
		h.ErrorResponse(err, "get public sessions")
		return
	}
	h.JSONResponse(SessionsListResponse{Items: sessionsToDTO(result.Sessions), Limit: result.Limit, Offset: result.Offset}, http.StatusOK)
}
