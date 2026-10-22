package booking_transport

import (
	"encoding/json/v2"
	"mime"
	"net/http"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
	core_logger "github.com/poponyas/TicketRush/internal/core/logger"
	core_response "github.com/poponyas/TicketRush/internal/core/response"
	core_transport "github.com/poponyas/TicketRush/internal/core/transport"
	booking_service "github.com/poponyas/TicketRush/internal/feature/booking/service"
)

const maxCreateBookingBody int64 = 16 << 10

func (t *HTTPBookingTransport) CreateBooking(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := core_logger.FromContext(ctx)
	responseHandler := core_response.NewHTTPResponseHandler(log, rw)
	identity, ok := auth.FromContext(ctx)
	if !ok || identity.UserID == uuid.Nil {
		responseHandler.ErrorResponse(core_errors.ErrUnauthenticated, "authentication required")
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
	r.Body = http.MaxBytesReader(rw, r.Body, maxCreateBookingBody)
	defer r.Body.Close()

	var request CreateBookingRequest
	if err := json.UnmarshalRead(r.Body, &request, json.RejectUnknownMembers(true)); err != nil {
		core_response.DecodeError(responseHandler, err)
		return
	}

	response, err := t.bookingService.CreateBooking(ctx, booking_service.CreateBookingInput{
		EventID:      eventID,
		SessionID:    sessionID,
		TicketTypeID: request.TicketTypeID,
		UserID:       identity.UserID,
		Quantity:     request.Quantity,
	})
	if err != nil {
		responseHandler.ErrorResponse(err, "create booking")
		return
	}
	responseHandler.JSONResponse(
		bookingResponse(response),
		http.StatusCreated,
	)
}
