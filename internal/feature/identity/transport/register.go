package auth_http_transport

import (
	"encoding/json/v2"
	"mime"
	"net/http"

	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
	core_logger "github.com/poponyas/TicketRush/internal/core/logger"
	core_response "github.com/poponyas/TicketRush/internal/core/response"
	auth_service "github.com/poponyas/TicketRush/internal/feature/identity/service"
)

const maxRegisterBody int64 = 16 << 10

func (t *HTTPAuthTransport) Register(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := core_logger.FromContext(ctx)
	responseHandler := core_response.NewHTTPResponseHandler(logger, rw)

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		responseHandler.ErrorResponse(core_errors.ErrUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	r.Body = http.MaxBytesReader(rw, r.Body, maxRegisterBody)
	defer r.Body.Close()

	var request RegisterRequest
	if err := json.UnmarshalRead(r.Body, &request, json.RejectUnknownMembers(true)); err != nil {
		core_response.DecodeError(responseHandler, err)
		return
	}

	input := auth_service.RegisterInput{
		Email:    request.Email,
		Password: request.Password,
	}

	result, err := t.authService.Register(ctx, input)
	if err != nil {
		responseHandler.ErrorResponse(err, "register error")
		return
	}

	responseHandler.JSONResponse(RegisterResponse{ID: result.ID, Email: result.Email}, http.StatusCreated)
}
