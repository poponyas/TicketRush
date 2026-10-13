package venue_http_tranpsort

import (
	"encoding/json/v2"
	"mime"
	"net/http"

	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
	core_logger "github.com/poponyas/TicketRush/internal/core/logger"
	core_response "github.com/poponyas/TicketRush/internal/core/response"
)

const maxByteBody = 1 << 20

func (h *HTTPVenueTransport) CreateVenue(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := core_logger.FromContext(ctx)
	responseHandler := core_response.NewHTTPResponseHandler(log, rw)
	identity, ok := auth.FromContext(ctx)
	if !ok {
		responseHandler.ErrorResponse(core_errors.ErrUnauthenticated, "authentication required")
		return
	}
	if !identity.HasRole(domain.RoleVenueOwner) {
		responseHandler.ErrorResponse(core_errors.ErrForbidden, "venue owner access required")
		return
	}

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		responseHandler.ErrorResponse(core_errors.ErrUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(rw, r.Body, maxByteBody)
	defer r.Body.Close()

	var request CreateVenueRequest
	if err := json.UnmarshalRead(r.Body, &request, json.RejectUnknownMembers(true)); err != nil {
		core_response.DecodeError(responseHandler, err)
		return
	}

	response, err := h.venueService.CreateVenue(ctx, request.Name, request.City, request.Address)
	if err != nil {
		responseHandler.ErrorResponse(err, "create error")
		return
	}

	responseHandler.JSONResponse(venueResponse(response), http.StatusCreated)
}
