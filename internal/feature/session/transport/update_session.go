package session_transport

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"mime"
	"net/http"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
	core_logger "github.com/poponyas/TicketRush/internal/core/logger"
	core_response "github.com/poponyas/TicketRush/internal/core/response"
	core_transport "github.com/poponyas/TicketRush/internal/core/transport"
	session_service "github.com/poponyas/TicketRush/internal/feature/session/service"
)

func (t *HTTPSessionTransport) UpdateSession(rw http.ResponseWriter, r *http.Request) {
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

	eventID, err := core_transport.GetIDInPath(r, "event_id")
	if err != nil {
		responseHandler.ErrorResponse(err, "get event ID")
		return
	}
	sessionID, err := core_transport.GetIDInPath(r, "session_id")
	if err != nil {
		responseHandler.ErrorResponse(err, "get session ID")
		return
	}

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		responseHandler.ErrorResponse(core_errors.ErrUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(rw, r.Body, maxSessionBody)
	defer r.Body.Close()
	var updateRequest UpdateSessionRequest
	if err := json.UnmarshalRead(r.Body, &updateRequest, json.RejectUnknownMembers(true)); err != nil {
		core_response.DecodeError(responseHandler, err)
		return
	}

	input := session_service.UpdateSessionInput{
		StartsAt:  updateRequest.StartsAt,
		EndsAt:    updateRequest.EndsAt,
		EndsAtSet: updateRequest.EndsAtSet,
	}

	response, err := t.sessionService.UpdateSession(ctx, input, eventID, sessionID, identity.OrganizerID, identity.UserID)
	if err != nil {
		responseHandler.ErrorResponse(err, "update session")
		return
	}

	responseHandler.JSONResponse(domainSessionToDTO(response), http.StatusOK)
}

func (u *UpdateSessionRequest) UnmarshalJSON(data []byte) error {
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if len(fields) == 0 {
		return fmt.Errorf("request must be a nonempty JSON object")
	}
	for key, value := range fields {
		switch key {
		case "starts_at":
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return fmt.Errorf("starts_at must not be null")
			}
		case "ends_at":
		default:
			return fmt.Errorf("unknown field %q", key)
		}
	}
	type plain UpdateSessionRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded, json.RejectUnknownMembers(true)); err != nil {
		return err
	}
	*u = UpdateSessionRequest(decoded)
	_, u.EndsAtSet = fields["ends_at"]
	return nil
}
