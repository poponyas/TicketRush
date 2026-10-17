package session_transport

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
	transport "github.com/poponyas/TicketRush/internal/core/transport"
)

func (t *HTTPSessionTransport) GetSessions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	res := response.NewHTTPResponseHandler(corelogger.FromContext(ctx), w)
	identity, ok := auth.FromContext(ctx)
	if !ok {
		res.ErrorResponse(coreerrors.ErrUnauthenticated, "authentication required")
		return
	}
	if !identity.HasRole(domain.RoleOrganizer) || identity.OrganizerID == uuid.Nil {
		res.ErrorResponse(coreerrors.ErrForbidden, "organizer access required")
		return
	}
	eventID, err := transport.GetIDInPath(r, "event_id")
	if err != nil {
		res.ErrorResponse(err, "get event ID")
		return
	}
	limit, err := transport.GetQueryByURL(r, "limit")
	if err != nil {
		res.ErrorResponse(err, "parse session limit")
		return
	}
	offset, err := transport.GetQueryByURL(r, "offset")
	if err != nil {
		res.ErrorResponse(err, "parse session offset")
		return
	}
	page, err := t.sessionService.GetSessions(ctx, eventID, identity.OrganizerID, identity.UserID, limit, offset)
	if err != nil {
		res.ErrorResponse(err, "list sessions")
		return
	}
	res.JSONResponse(SessionsListResponse{Items: sessionsToDTO(page.Sessions), Limit: page.Limit, Offset: page.Offset}, http.StatusOK)
}
