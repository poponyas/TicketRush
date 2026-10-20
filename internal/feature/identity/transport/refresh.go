package auth_http_transport

import (
	"encoding/json/v2"
	"mime"
	"net/http"

	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
	service "github.com/poponyas/TicketRush/internal/feature/identity/service"
)

const maxRefreshBody int64 = 16 << 10

func (t *HTTPAuthTransport) Refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	handler := response.NewHTTPResponseHandler(corelogger.FromContext(ctx), w)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		handler.ErrorResponse(coreerrors.ErrUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRefreshBody)
	defer r.Body.Close()
	var request RefreshRequest
	if err := json.UnmarshalRead(r.Body, &request, json.RejectUnknownMembers(true)); err != nil {
		response.DecodeError(handler, err)
		return
	}
	result, err := t.authService.Refresh(ctx, service.RefreshInput{RefreshToken: request.RefreshToken})
	if err != nil {
		handler.ErrorResponse(err, "refresh failed")
		return
	}
	handler.JSONResponse(RefreshResponse{AccessToken: result.AccessToken, RefreshToken: result.RefreshToken}, http.StatusOK)
}
