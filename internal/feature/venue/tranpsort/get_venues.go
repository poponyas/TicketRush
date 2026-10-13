package venue_http_tranpsort

import (
	"net/http"

	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
	transport "github.com/poponyas/TicketRush/internal/core/transport"
)

func (h *HTTPVenueTransport) GetVenues(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	res := response.NewHTTPResponseHandler(corelogger.FromContext(ctx), w)
	limit, err := transport.GetQueryByURL(r, "limit")
	if err != nil {
		res.ErrorResponse(err, "parse venue limit")
		return
	}
	offset, err := transport.GetQueryByURL(r, "offset")
	if err != nil {
		res.ErrorResponse(err, "parse venue offset")
		return
	}
	page, err := h.venueService.GetVenues(ctx, limit, offset)
	if err != nil {
		res.ErrorResponse(err, "list venues")
		return
	}
	res.JSONResponse(VenuesListResponse{Items: venuesResponse(page.Venues), Limit: page.Limit, Offset: page.Offset}, http.StatusOK)
}
