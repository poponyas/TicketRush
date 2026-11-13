package core_response

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	"go.uber.org/zap"
)

type HTTPResponseHandler struct {
	rw     http.ResponseWriter
	logger *corelogger.Logger
}

func NewHTTPResponseHandler(log *corelogger.Logger, w http.ResponseWriter) *HTTPResponseHandler {
	return &HTTPResponseHandler{rw: w, logger: log}
}

type ErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

func (h *HTTPResponseHandler) JSONResponse(v any, statusCode int) {
	data, err := json.Marshal(v)
	if err != nil {
		h.logger.Error("marshal HTTP response", zap.Error(err))
		data = []byte(`{"code":"internal_error","message":"internal server error"}`)
		statusCode = http.StatusInternalServerError
	}
	h.rw.Header().Set("Content-Type", "application/json")
	h.rw.WriteHeader(statusCode)
	if _, err := h.rw.Write(append(data, '\n')); err != nil {
		h.logger.Error("write HTTP response", zap.Error(err))
	}
}
func (h *HTTPResponseHandler) NoContentResponse() { h.rw.WriteHeader(http.StatusNoContent) }
func (h *HTTPResponseHandler) ErrorResponse(err error, msg string) {
	status := http.StatusInternalServerError
	code, publicMessage := "internal_error", "internal server error"
	switch {
	case errors.Is(err, coreerrors.ErrUnavailable):
		status = http.StatusServiceUnavailable
		code = "unavailable"
		publicMessage = "service temporarily unavailable"
	case errors.Is(err, coreerrors.ErrInvalidArgument):
		status = http.StatusBadRequest
		code = "invalid_argument"
		publicMessage = "invalid request"
	case errors.Is(err, coreerrors.ErrUnauthenticated):
		status = http.StatusUnauthorized
		code = "unauthenticated"
		publicMessage = "authentication required"
	case errors.Is(err, coreerrors.ErrForbidden):
		status = http.StatusForbidden
		code = "forbidden"
		publicMessage = "access denied"
	case errors.Is(err, coreerrors.ErrNotFound):
		status = http.StatusNotFound
		code = "not_found"
		publicMessage = "resource not found"
	case errors.Is(err, coreerrors.ErrConflict):
		status = http.StatusConflict
		code = "conflict"
		publicMessage = "operation conflicts with current state"
	case errors.Is(err, coreerrors.ErrPayloadTooLarge):
		status = http.StatusRequestEntityTooLarge
		code = "payload_too_large"
		publicMessage = "request body is too large"
	case errors.Is(err, coreerrors.ErrUnsupportedMediaType):
		status = http.StatusUnsupportedMediaType
		code = "unsupported_media_type"
		publicMessage = "Content-Type must be application/json"
	}
	if status >= 500 {
		h.logger.Error(msg, zap.Error(err))
	} else {
		h.logger.Warn(msg, zap.Error(err))
	}
	h.JSONResponse(ErrorResponse{Code: code, Message: publicMessage, RequestID: h.rw.Header().Get(middleware.RequestIDHeader)}, status)
}

func DecodeError(h *HTTPResponseHandler, err error) {
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		h.ErrorResponse(coreerrors.ErrPayloadTooLarge, "request too large")
		return
	}
	h.ErrorResponse(fmt.Errorf("%w: %v", coreerrors.ErrInvalidArgument, err), "invalid JSON body")
}
