package ticket_type_transport

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
)

const maxCreateTicketTypeBody int64 = 16 << 10

func (t *HTTPTicketType) CreateTicketType(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h := core_response.NewHTTPResponseHandler(core_logger.FromContext(ctx), rw)
	identity, ok := auth.FromContext(ctx)
	if !ok {
		h.ErrorResponse(core_errors.ErrUnauthenticated, "authentication required")
		return
	}
	if !identity.HasRole(domain.RoleOrganizer) || identity.OrganizerID == uuid.Nil {
		h.ErrorResponse(core_errors.ErrForbidden, "organizer access required")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		h.ErrorResponse(core_errors.ErrUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(rw, r.Body, maxCreateTicketTypeBody)
	defer r.Body.Close()

	eventID, err := core_transport.GetIDInPath(r, "event_id")
	if err != nil {
		h.ErrorResponse(err, "can't get eventID")
		return
	}

	sessionID, err := core_transport.GetIDInPath(r, "session_id")
	if err != nil {
		h.ErrorResponse(err, "can't get sessionID")
		return
	}

	var request TicketTypeRequest
	if err := json.UnmarshalRead(r.Body, &request, json.RejectUnknownMembers(true)); err != nil {
		core_response.DecodeError(h, err)
		return
	}

	response, err := t.ticketTypeService.CreateTicketType(
		ctx,
		eventID,
		sessionID,
		identity.OrganizerID,
		identity.UserID,
		request.Name,
		request.Price,
		request.Capacity,
	)
	if err != nil {
		h.ErrorResponse(err, "create ticket type")
		return
	}

	h.JSONResponse(domainToTicketTypeResponse(response), http.StatusCreated)
}
