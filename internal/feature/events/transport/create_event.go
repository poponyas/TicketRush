package events_transport

import (
	"encoding/json/v2"
	"mime"
	"net/http"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	core_response "github.com/poponyas/TicketRush/internal/core/response"
	response "github.com/poponyas/TicketRush/internal/core/response"
	service "github.com/poponyas/TicketRush/internal/feature/events/service"
)

const maxCreateEventBody = 1 << 20

func (t *EventsHTTPTransport) CreateEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h := response.NewHTTPResponseHandler(corelogger.FromContext(ctx), w)
	identity, ok := auth.FromContext(ctx)
	if !ok {
		h.ErrorResponse(coreerrors.ErrUnauthenticated, "authentication required")
		return
	}
	if !identity.HasRole(domain.RoleOrganizer) || identity.OrganizerID == uuid.Nil {
		h.ErrorResponse(coreerrors.ErrForbidden, "organizer access required")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		h.ErrorResponse(coreerrors.ErrUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxCreateEventBody)
	defer r.Body.Close()
	var request EventCreateRequest
	// UnmarshalRead requires exactly one JSON value and consumes the whole body.
	if err := json.UnmarshalRead(r.Body, &request, json.RejectUnknownMembers(true)); err != nil {
		core_response.DecodeError(h, err)
		return
	}
	event, err := t.eventService.CreateEvent(ctx, service.CreateEventInput{
		UserID: identity.UserID, OrganizerID: identity.OrganizerID,
		Title: request.Title, Description: request.Description, Category: request.Category,
	})
	if err != nil {
		h.ErrorResponse(err, "create event")
		return
	}
	w.Header().Set("Location", "/api/v1/organizer/events/"+event.ID.String())
	h.JSONResponse(eventResponse(event), http.StatusCreated)
}
