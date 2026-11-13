package tickets_transport

import (
	"context"
	"encoding/json/v2"
	"mime"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	logger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
	transport "github.com/poponyas/TicketRush/internal/core/transport"
	service "github.com/poponyas/TicketRush/internal/feature/tickets/service"
)

type Service interface {
	Get(context.Context, uuid.UUID, uuid.UUID) (service.Detail, error)
	List(context.Context, uuid.UUID, int, int) ([]domain.Ticket, error)
	QR(context.Context, uuid.UUID, uuid.UUID) ([]byte, error)
	CheckIn(context.Context, string, uuid.UUID, uuid.UUID) (domain.Ticket, error)
}
type Transport struct{ service Service }

func New(s Service) *Transport { return &Transport{service: s} }

type TicketResponse struct {
	ID           uuid.UUID           `json:"id"`
	BookingID    uuid.UUID           `json:"booking_id"`
	SessionID    uuid.UUID           `json:"session_id"`
	TicketTypeID uuid.UUID           `json:"ticket_type_id"`
	Status       domain.TicketStatus `json:"status"`
	CreatedAt    time.Time           `json:"created_at"`
	UsedAt       *time.Time          `json:"used_at,omitempty"`
	QRPayload    string              `json:"qr_payload,omitempty"`
}

func dto(ticket domain.Ticket) TicketResponse {
	return TicketResponse{ID: ticket.ID, BookingID: ticket.BookingID, SessionID: ticket.SessionID, TicketTypeID: ticket.TicketTypeID, Status: ticket.Status, CreatedAt: ticket.CreatedAt, UsedAt: ticket.UsedAt}
}
func identity(w http.ResponseWriter, r *http.Request) (auth.Identity, *response.HTTPResponseHandler, bool) {
	h := response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	i, ok := auth.FromContext(r.Context())
	if !ok {
		h.ErrorResponse(coreerrors.ErrUnauthenticated, "authentication required")
	}
	return i, h, ok
}
func (t *Transport) Get(w http.ResponseWriter, r *http.Request) {
	i, h, ok := identity(w, r)
	if !ok {
		return
	}
	id, err := transport.GetIDInPath(r, "ticket_id")
	if err != nil {
		h.ErrorResponse(err, "ticket ID")
		return
	}
	detail, err := t.service.Get(r.Context(), id, i.UserID)
	if err != nil {
		h.ErrorResponse(err, "own ticket")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	result := dto(detail.Ticket)
	result.QRPayload = detail.QRPayload
	h.JSONResponse(result, http.StatusOK)
}
func (t *Transport) List(w http.ResponseWriter, r *http.Request) {
	i, h, ok := identity(w, r)
	if !ok {
		return
	}
	limit, offset := 20, 0
	l, err := transport.GetQueryByURL(r, "limit")
	if err != nil {
		h.ErrorResponse(err, "limit")
		return
	}
	if l != nil {
		limit = *l
	}
	o, err := transport.GetQueryByURL(r, "offset")
	if err != nil {
		h.ErrorResponse(err, "offset")
		return
	}
	if o != nil {
		offset = *o
	}
	tickets, err := t.service.List(r.Context(), i.UserID, limit, offset)
	if err != nil {
		h.ErrorResponse(err, "own tickets")
		return
	}
	items := make([]TicketResponse, 0, len(tickets))
	for _, ticket := range tickets {
		items = append(items, dto(ticket))
	}
	w.Header().Set("Cache-Control", "private, no-store")
	h.JSONResponse(struct {
		Items  []TicketResponse `json:"items"`
		Limit  int              `json:"limit"`
		Offset int              `json:"offset"`
	}{items, limit, offset}, http.StatusOK)
}
func (t *Transport) QR(w http.ResponseWriter, r *http.Request) {
	i, h, ok := identity(w, r)
	if !ok {
		return
	}
	id, err := transport.GetIDInPath(r, "ticket_id")
	if err != nil {
		h.ErrorResponse(err, "ticket ID")
		return
	}
	png, err := t.service.QR(r.Context(), id, i.UserID)
	if err != nil {
		h.ErrorResponse(err, "ticket QR")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := w.Write(png); err != nil {
		logger.FromContext(r.Context()).Error("write ticket QR")
	}
}
func (t *Transport) CheckIn(w http.ResponseWriter, r *http.Request) {
	i, h, ok := identity(w, r)
	if !ok {
		return
	}
	session, err := transport.GetIDInPath(r, "session_id")
	if err != nil {
		h.ErrorResponse(err, "check-in session")
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		h.ErrorResponse(coreerrors.ErrUnsupportedMediaType, "check-in JSON required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	defer r.Body.Close()
	var input struct {
		Token string `json:"token"`
	}
	if err := json.UnmarshalRead(r.Body, &input, json.RejectUnknownMembers(true)); err != nil {
		response.DecodeError(h, err)
		return
	}
	ticket, err := t.service.CheckIn(r.Context(), input.Token, session, i.UserID)
	if err != nil {
		h.ErrorResponse(err, "check-in ticket")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	h.JSONResponse(dto(ticket), http.StatusOK)
}
