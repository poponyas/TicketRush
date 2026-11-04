package booking_transport

import (
	"encoding/json/v2"
	"errors"
	"mime"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
	core_logger "github.com/poponyas/TicketRush/internal/core/logger"
	core_response "github.com/poponyas/TicketRush/internal/core/response"
	core_transport "github.com/poponyas/TicketRush/internal/core/transport"
	booking_service "github.com/poponyas/TicketRush/internal/feature/booking/service"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

const maxCreateBookingBody int64 = 16 << 10

func (t *HTTPBookingTransport) CreateBooking(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := core_logger.FromContext(ctx).With(
		zap.String("operation", "booking.create"),
	)
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

	log = log.With(
		zap.String("event_id", eventID.String()),
		zap.String("session_id", sessionID.String()),
		zap.String("ticket_type_id", request.TicketTypeID.String()),
		zap.Int("quantity", request.Quantity),
	)

	ctx, span := otel.Tracer("ticketrush/booking").
		Start(ctx, "booking.create")
	defer span.End()

	spanContext := span.SpanContext()

	log = log.With(
		zap.String("trace_id", spanContext.TraceID().String()),
		zap.String("span_id", spanContext.SpanID().String()),
	)

	startedAt := time.Now()
	response, err := t.bookingService.CreateBooking(ctx, booking_service.CreateBookingInput{
		EventID:      eventID,
		SessionID:    sessionID,
		TicketTypeID: request.TicketTypeID,
		UserID:       identity.UserID,
		Quantity:     request.Quantity,
	})
	durationMS := time.Since(startedAt).Seconds() * 1000

	outcome := "error"
	switch {
	case err == nil:
		outcome = "success"
	case errors.Is(err, core_errors.ErrInvalidArgument),
		errors.Is(err, core_errors.ErrUnauthenticated),
		errors.Is(err, core_errors.ErrForbidden),
		errors.Is(err, core_errors.ErrNotFound),
		errors.Is(err, core_errors.ErrConflict),
		errors.Is(err, core_errors.ErrPayloadTooLarge),
		errors.Is(err, core_errors.ErrUnsupportedMediaType):
		outcome = "rejected"
	}
	span.SetAttributes(
		attribute.String("booking.outcome", outcome),
		attribute.Int("booking.quantity", request.Quantity),
	)

	if outcome == "error" {
		span.RecordError(err)
		span.SetStatus(codes.Error, "booking creation failed")
	}

	t.metrics.Attempts.WithLabelValues(outcome).Inc()
	t.metrics.Duration.WithLabelValues(outcome).Observe(durationMS / 1000)

	log = log.With(
		zap.String("outcome", outcome),
		zap.Float64("duration_ms", durationMS),
	)
	if err == nil {
		log = log.With(zap.String("booking_id", response.ID.String()))
	}
	responseHandler = core_response.NewHTTPResponseHandler(log, rw)
	if err != nil {
		responseHandler.ErrorResponse(err, "create booking")
		return
	}

	log.Info("booking created")
	responseHandler.JSONResponse(
		bookingResponse(response),
		http.StatusCreated,
	)
}
