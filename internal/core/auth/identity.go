package auth

import (
	"context"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
	"net/http"
)

type Identity struct {
	UserID      uuid.UUID
	SessionID   uuid.UUID
	OrganizerID uuid.UUID
	Roles       []domain.Role
}

func (i Identity) HasRole(role domain.Role) bool {
	for _, current := range i.Roles {
		if current == role {
			return true
		}
	}
	return false
}

type identityKey struct{}

// WithIdentity is called only by trusted authentication middleware.
func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, identity)
}
func FromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityKey{}).(Identity)
	return identity, ok && identity.UserID != uuid.Nil
}

// DevIdentity is a local-only bridge until JWT/OIDC authentication is implemented.
// It never reads a role or organizer ID from client cookies or headers.
func DevIdentity(identity Identity) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), identity)))
		})
	}
}
func RequireOrganizer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := FromContext(r.Context())
		h := response.NewHTTPResponseHandler(corelogger.FromContext(r.Context()), w)
		if !ok {
			h.ErrorResponse(coreerrors.ErrUnauthenticated, "authentication required")
			return
		}
		if !identity.HasRole(domain.RoleOrganizer) || identity.OrganizerID == uuid.Nil {
			h.ErrorResponse(coreerrors.ErrForbidden, "organizer access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireVenueOwner checks the creation permission, not an organizer membership.
func RequireVenueOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := FromContext(r.Context())
		h := response.NewHTTPResponseHandler(corelogger.FromContext(r.Context()), w)
		if !ok {
			h.ErrorResponse(coreerrors.ErrUnauthenticated, "authentication required")
			return
		}
		if !identity.HasRole(domain.RoleVenueOwner) {
			h.ErrorResponse(coreerrors.ErrForbidden, "venue owner access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
