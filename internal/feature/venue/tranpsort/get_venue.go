package venue_http_tranpsort

import (
	"net/http"

	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
	transport "github.com/poponyas/TicketRush/internal/core/transport"
)

func (h *HTTPVenueTransport) GetVenue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	res := response.NewHTTPResponseHandler(corelogger.FromContext(ctx), w)
	id, err := transport.GetIDInPath(r, "venue_id")
	if err != nil {
		res.ErrorResponse(err, "get venue ID")
		return
	}
	venue, err := h.venueService.GetVenue(ctx, id)
	if err != nil {
		res.ErrorResponse(err, "get venue")
		return
	}
	res.JSONResponse(venueResponse(venue), http.StatusOK)
}
