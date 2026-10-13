package events_transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	core_logger "github.com/poponyas/TicketRush/internal/core/logger"
	core_response "github.com/poponyas/TicketRush/internal/core/response"
	core_transport "github.com/poponyas/TicketRush/internal/core/transport"
	service "github.com/poponyas/TicketRush/internal/feature/events/service"
)

type UpdateEventRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Category    *string `json:"category"`
}

func (h *EventsHTTPTransport) UpdateEvent(rw http.ResponseWriter, r *http.Request) {
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

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		responseHandler.ErrorResponse(coreerrors.ErrUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(rw, r.Body, maxCreateEventBody)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	var update UpdateEventRequest
	if err := decoder.Decode(&update); err != nil {
		decodeError(responseHandler, err)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		decodeError(responseHandler, err)
		return
	}

	eventID, err := core_transport.GetIDInPath(r, "event_id")
	if err != nil {
		responseHandler.ErrorResponse(err, "get id error")
		return
	}

	input := service.UpdateEventInput{
		Name:        update.Name,
		Description: update.Description,
		Category:    update.Category,
	}
	response, err := h.eventService.UpdateEvent(ctx, input, eventID, identity.OrganizerID, identity.UserID)
	if err != nil {
		responseHandler.ErrorResponse(err, "update event")
		return
	}

	responseHandler.JSONResponse(
		eventResponse(response),
		http.StatusOK,
	)
}

func (u *UpdateEventRequest) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}

	if len(fields) == 0 {
		return fmt.Errorf("request must be a nonempty JSON object")
	}

	for key, value := range fields {
		switch key {
		case "name", "description", "category":
		default:
			return fmt.Errorf("unknown field %q", key)
		}

		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("field %q must not be null", key)
		}
	}

	type plain UpdateEventRequest

	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*u = UpdateEventRequest(decoded)
	return nil
}
