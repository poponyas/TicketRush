package posters_transport

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	logger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
	transport "github.com/poponyas/TicketRush/internal/core/transport"
	service "github.com/poponyas/TicketRush/internal/feature/posters/service"
	"io"
	"mime"
	"net/http"
)

type Service interface {
	Upload(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, []byte) (string, error)
	Get(context.Context, uuid.UUID) ([]byte, string, error)
}
type Transport struct{ service Service }

func New(s Service) *Transport { return &Transport{service: s} }
func (t *Transport) Upload(w http.ResponseWriter, r *http.Request) {
	h := response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	i, ok := auth.FromContext(r.Context())
	if !ok {
		h.ErrorResponse(coreerrors.ErrUnauthenticated, "authentication required")
		return
	}
	if !i.HasRole(domain.RoleOrganizer) || i.OrganizerID == uuid.Nil {
		h.ErrorResponse(coreerrors.ErrForbidden, "organizer required")
		return
	}
	id, err := transport.GetIDInPath(r, "event_id")
	if err != nil {
		h.ErrorResponse(err, "poster event")
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || (media != "image/png" && media != "image/jpeg") {
		h.ErrorResponse(coreerrors.ErrUnsupportedMediaType, "PNG or JPEG required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, service.MaxSize)
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			err = coreerrors.ErrPayloadTooLarge
		} else {
			err = coreerrors.ErrInvalidArgument
		}
		h.ErrorResponse(err, "read poster")
		return
	}
	url, err := t.service.Upload(r.Context(), id, i.OrganizerID, i.UserID, data)
	if err != nil {
		h.ErrorResponse(err, "upload poster")
		return
	}
	h.JSONResponse(struct {
		URL string `json:"url"`
	}{url}, http.StatusOK)
}
func (t *Transport) Get(w http.ResponseWriter, r *http.Request) {
	h := response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
	id, err := transport.GetIDInPath(r, "event_id")
	if err != nil {
		h.ErrorResponse(err, "poster event")
		return
	}
	data, content, err := t.service.Get(r.Context(), id)
	if err != nil {
		h.ErrorResponse(err, "get public poster")
		return
	}
	w.Header().Set("Content-Type", content)
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := w.Write(data); err != nil {
		logger.FromContext(r.Context()).Error("write poster")
	}
}
