package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	response "github.com/poponyas/TicketRush/internal/core/response"
)

type AccessTokenVerifier interface {
	VerifyAccess(rawToken string) (uuid.UUID, uuid.UUID, error)
}

type ActiveSessionChecker interface {
	CheckActiveSession(ctx context.Context, sessionID, userID uuid.UUID) error
}

func Authenticate(verifier AccessTokenVerifier, sessions ActiveSessionChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			unauthenticated := func() {
				w.Header().Set("WWW-Authenticate", "Bearer")
				h := response.NewHTTPResponseHandler(corelogger.FromContext(r.Context()), w)
				h.ErrorResponse(coreerrors.ErrUnauthenticated, "authentication required")
			}

			values := r.Header.Values("Authorization")
			if len(values) != 1 {
				unauthenticated()
				return
			}
			parts := strings.Fields(values[0])
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				unauthenticated()
				return
			}

			userID, sessionID, err := verifier.VerifyAccess(parts[1])
			if err != nil || userID == uuid.Nil || sessionID == uuid.Nil {
				unauthenticated()
				return
			}
			if err := sessions.CheckActiveSession(r.Context(), sessionID, userID); err != nil {
				if errors.Is(err, coreerrors.ErrUnauthenticated) {
					unauthenticated()
				} else {
					h := response.NewHTTPResponseHandler(corelogger.FromContext(r.Context()), w)
					h.ErrorResponse(err, "check active auth session")
				}
				return
			}
			identity := Identity{UserID: userID, SessionID: sessionID}
			ctx := WithIdentity(r.Context(), identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
