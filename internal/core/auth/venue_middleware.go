package auth

import (
	"context"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	logger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
	"net/http"
)

type VenueOwnerChecker interface {
	CheckVenueOwnerAccess(context.Context, uuid.UUID) error
}

// SelectVenueOwner loads the global permission; a JWT authenticates the user
// but neither a client-supplied role nor organizer membership grants this right.
func SelectVenueOwner(checker VenueOwnerChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)
			i, ok := FromContext(r.Context())
			if !ok {
				h.ErrorResponse(coreerrors.ErrUnauthenticated, "authentication required")
				return
			}
			if err := checker.CheckVenueOwnerAccess(r.Context(), i.UserID); err != nil {
				h.ErrorResponse(err, "check venue owner")
				return
			}
			i.Roles = append(i.Roles, domain.RoleVenueOwner)
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), i)))
		})
	}
}
