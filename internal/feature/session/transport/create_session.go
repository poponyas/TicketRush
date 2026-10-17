package session_transport

import (
	"encoding/json/v2"
	"mime"
	"net/http"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
	core_logger "github.com/poponyas/TicketRush/internal/core/logger"
	core_response "github.com/poponyas/TicketRush/internal/core/response"
	core_transport "github.com/poponyas/TicketRush/internal/core/transport"
	service "github.com/poponyas/TicketRush/internal/feature/session/service"
)

const maxSessionBody = 1 << 20

func (t *HTTPSessionTransport) CreateSession(rw http.ResponseWriter, r *http.Request) {
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

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		responseHandler.ErrorResponse(core_errors.ErrUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(rw, r.Body, maxSessionBody)
	defer r.Body.Close()
	var request CreateSessionRequest
	if err := json.UnmarshalRead(r.Body, &request, json.RejectUnknownMembers(true)); err != nil {
		core_response.DecodeError(responseHandler, err)
		return
	}

	eventID, err := core_transport.GetIDInPath(r, "event_id")
	if err != nil {
		responseHandler.ErrorResponse(err, "get event_id from path")
		return
	}

	input := service.CreateSessionInput{
		VenueID:      request.VenueID,
		StartsAt:     request.StartsAt,
		EndsAt:       request.EndsAt,
		SalesStartAt: request.SalesStartAt,
		SalesEndAt:   request.SalesEndAt,
	}
	session, err := t.sessionService.CreateSession(ctx, input, identity.OrganizerID, identity.UserID, eventID)
	if err != nil {
		responseHandler.ErrorResponse(err, "create session")
		return
	}

	responseHandler.JSONResponse(domainSessionToDTO(session), http.StatusCreated)
}
