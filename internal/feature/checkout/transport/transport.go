package checkout_transport

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	logger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
	transport "github.com/poponyas/TicketRush/internal/core/transport"
)

type Service interface {
	Confirm(context.Context, uuid.UUID, uuid.UUID) (domain.Booking, error)
}
type Transport struct{ service Service }

func New(s Service) *Transport { return &Transport{service: s} }
func (t *Transport) Confirm(w http.ResponseWriter, r *http.Request) {
	h := response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	identity, ok := auth.FromContext(r.Context())
	if !ok {
		h.ErrorResponse(coreerrors.ErrUnauthenticated, "authentication required")
		return
	}
	id, err := transport.GetIDInPath(r, "booking_id")
	if err != nil {
		h.ErrorResponse(err, "booking ID")
		return
	}
	b, err := t.service.Confirm(r.Context(), id, identity.UserID)
	if err != nil {
		h.ErrorResponse(err, "mock checkout")
		return
	}
	h.JSONResponse(struct {
		ID          uuid.UUID            `json:"id"`
		SessionID   uuid.UUID            `json:"session_id"`
		Status      domain.BookingStatus `json:"status"`
		PaymentMode string               `json:"payment_mode"`
	}{b.ID, b.SessionID, b.Status, "mock"}, http.StatusOK)
}
