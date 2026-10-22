package booking_transport

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
	coretransport "github.com/poponyas/TicketRush/internal/core/transport"
)

func (t *HTTPBookingTransport) CancelBooking(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h := response.NewHTTPResponseHandler(corelogger.FromContext(ctx), rw)
	identity, ok := auth.FromContext(ctx)
	if !ok || identity.UserID == uuid.Nil {
		h.ErrorResponse(coreerrors.ErrUnauthenticated, "authentication required")
		return
	}
	bookingID, err := coretransport.GetIDInPath(r, "booking_id")
	if err != nil {
		h.ErrorResponse(err, "get booking ID")
		return
	}
	booking, err := t.bookingService.CancelBooking(ctx, bookingID, identity.UserID)
	if err != nil {
		h.ErrorResponse(err, "cancel own booking")
		return
	}
	h.JSONResponse(bookingResponse(booking), http.StatusOK)
}
