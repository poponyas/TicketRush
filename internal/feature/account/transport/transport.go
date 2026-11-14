package account_transport

import (
	"context"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	logger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
	transport "github.com/poponyas/TicketRush/internal/core/transport"
	"github.com/poponyas/TicketRush/internal/feature/account"
	"net/http"
)

type Service interface {
	Profile(context.Context, uuid.UUID) (account.Profile, error)
	Bookings(context.Context, uuid.UUID, int, int) (account.BookingPage, error)
}
type Transport struct{ service Service }

func New(s Service) *Transport { return &Transport{service: s} }
func (t *Transport) Profile(w http.ResponseWriter, r *http.Request) {
	h := response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	w.Header().Set("Cache-Control", "private, no-store")
	i, ok := auth.FromContext(r.Context())
	if !ok {
		h.ErrorResponse(coreerrors.ErrUnauthenticated, "account identity")
		return
	}
	p, err := t.service.Profile(r.Context(), i.UserID)
	if err != nil {
		h.ErrorResponse(err, "account profile")
		return
	}
	h.JSONResponse(p, http.StatusOK)
}
func (t *Transport) Bookings(w http.ResponseWriter, r *http.Request) {
	h := response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	w.Header().Set("Cache-Control", "private, no-store")
	i, ok := auth.FromContext(r.Context())
	if !ok {
		h.ErrorResponse(coreerrors.ErrUnauthenticated, "account identity")
		return
	}
	limit, offset := 20, 0
	l, err := transport.GetQueryByURL(r, "limit")
	if err != nil {
		h.ErrorResponse(err, "booking page limit")
		return
	}
	o, err := transport.GetQueryByURL(r, "offset")
	if err != nil {
		h.ErrorResponse(err, "booking page offset")
		return
	}
	if l != nil {
		limit = *l
	}
	if o != nil {
		offset = *o
	}
	p, err := t.service.Bookings(r.Context(), i.UserID, limit, offset)
	if err != nil {
		h.ErrorResponse(err, "account bookings")
		return
	}
	h.JSONResponse(p, http.StatusOK)
}
