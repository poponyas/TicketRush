package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
)

type OrganizerAccessChecker interface {
	CheckOrganizerAccess(ctx context.Context, organizerID, userID uuid.UUID) error
}

func SelectOrganizer(checker OrganizerAccessChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler := response.NewHTTPResponseHandler(corelogger.FromContext(r.Context()), w)
			identity, ok := FromContext(r.Context())
			if !ok {
				handler.ErrorResponse(coreerrors.ErrUnauthenticated, "authentication required")
				return
			}
			values := r.Header.Values("X-Organizer-ID")
			if len(values) != 1 {
				handler.ErrorResponse(coreerrors.ErrInvalidArgument, "one X-Organizer-ID is required")
				return
			}
			organizerID, err := uuid.Parse(strings.TrimSpace(values[0]))
			if err != nil || organizerID == uuid.Nil {
				handler.ErrorResponse(coreerrors.ErrInvalidArgument, "invalid X-Organizer-ID")
				return
			}
			if err := checker.CheckOrganizerAccess(r.Context(), organizerID, identity.UserID); err != nil {
				handler.ErrorResponse(err, "check organizer access")
				return
			}
			identity.OrganizerID = organizerID
			identity.Roles = []domain.Role{domain.RoleOrganizer}
			ctx := WithIdentity(r.Context(), identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
