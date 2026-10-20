package auth_http_transport

import (
	"net/http"

	"github.com/poponyas/TicketRush/internal/core/auth"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
	core_logger "github.com/poponyas/TicketRush/internal/core/logger"
	core_response "github.com/poponyas/TicketRush/internal/core/response"
)

func (t *HTTPAuthTransport) Logout(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := core_logger.FromContext(ctx)
	responseHandler := core_response.NewHTTPResponseHandler(log, rw)

	identity, ok := auth.FromContext(ctx)
	if !ok {
		responseHandler.ErrorResponse(core_errors.ErrUnauthenticated, "authentication required")
		return
	}

	if err := t.authService.Logout(ctx, identity.SessionID, identity.UserID); err != nil {
		responseHandler.ErrorResponse(err, "logout error")
		return
	}

	responseHandler.NoContentResponse()
}
