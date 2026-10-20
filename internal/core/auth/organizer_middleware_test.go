package auth_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	"github.com/stretchr/testify/require"
)

type organizerCheckerFunc func(context.Context, uuid.UUID, uuid.UUID) error

func (f organizerCheckerFunc) CheckOrganizerAccess(ctx context.Context, org, user uuid.UUID) error {
	return f(ctx, org, user)
}

func TestSelectOrganizer(t *testing.T) {
	user, session, org := uuid.New(), uuid.New(), uuid.New()
	for _, tc := range []struct {
		name          string
		headers       []string
		authenticated bool
		checkErr      error
		status, calls int
	}{
		{name: "allowed", headers: []string{org.String()}, authenticated: true, status: 204, calls: 1},
		{name: "missing identity", headers: []string{org.String()}, status: 401},
		{name: "missing header", authenticated: true, status: 400},
		{name: "empty header", headers: []string{""}, authenticated: true, status: 400},
		{name: "invalid uuid", headers: []string{"bad"}, authenticated: true, status: 400},
		{name: "nil uuid", headers: []string{uuid.Nil.String()}, authenticated: true, status: 400},
		{name: "duplicate header", headers: []string{org.String(), org.String()}, authenticated: true, status: 400},
		{name: "not a member", headers: []string{org.String()}, authenticated: true, checkErr: fmt.Errorf("membership: %w", coreerrors.ErrForbidden), status: 403, calls: 1},
		{name: "database failure", headers: []string{org.String()}, authenticated: true, checkErr: errors.New("database unavailable"), status: 500, calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			called := false
			req := httptest.NewRequest(http.MethodGet, "/organizer", nil)
			if tc.authenticated {
				req = req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{UserID: user, SessionID: session, OrganizerID: uuid.New(), Roles: []domain.Role{domain.RoleController}}))
			}
			for _, value := range tc.headers {
				req.Header.Add("X-Organizer-ID", value)
			}
			checker := organizerCheckerFunc(func(ctx context.Context, orgID, userID uuid.UUID) error {
				calls++
				require.Equal(t, req.Context(), ctx)
				require.Equal(t, org, orgID)
				require.Equal(t, user, userID)
				return tc.checkErr
			})
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				identity, ok := auth.FromContext(r.Context())
				require.True(t, ok)
				require.Equal(t, user, identity.UserID)
				require.Equal(t, session, identity.SessionID)
				require.Equal(t, org, identity.OrganizerID)
				require.Equal(t, []domain.Role{domain.RoleOrganizer}, identity.Roles)
				w.WriteHeader(204)
			})
			w := httptest.NewRecorder()
			auth.SelectOrganizer(checker)(auth.RequireOrganizer(next)).ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.Equal(t, tc.calls, calls)
			require.Equal(t, tc.status == 204, called)
			require.NotContains(t, w.Body.String(), "database unavailable")
		})
	}
}
