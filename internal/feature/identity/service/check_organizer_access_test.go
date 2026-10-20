package auth_service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/identity/service"
	"github.com/stretchr/testify/require"
)

func TestCheckOrganizerAccess(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	databaseErr := errors.New("database unavailable")
	for _, tc := range []struct {
		name          string
		role          domain.Role
		repoErr, want error
		org, user     uuid.UUID
		calls         int
	}{
		{name: "organizer", role: domain.RoleOrganizer, org: org, user: user, calls: 1},
		{name: "controller", role: domain.RoleController, org: org, user: user, calls: 1, want: coreerrors.ErrForbidden},
		{name: "unrecognized role", role: "unknown", org: org, user: user, calls: 1, want: coreerrors.ErrForbidden},
		{name: "no membership", repoErr: fmt.Errorf("storage: %w", coreerrors.ErrNotFound), org: org, user: user, calls: 1, want: coreerrors.ErrForbidden},
		{name: "database error", repoErr: databaseErr, org: org, user: user, calls: 1, want: databaseErr},
		{name: "cancelled", repoErr: context.Canceled, org: org, user: user, calls: 1, want: context.Canceled},
		{name: "nil organizer", user: user, want: coreerrors.ErrInvalidArgument},
		{name: "nil user", org: org, want: coreerrors.ErrInvalidArgument},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			calls := 0
			repo := &repoStub{role: func(got context.Context, orgID, userID uuid.UUID) (domain.Role, error) {
				calls++
				require.Equal(t, ctx, got)
				require.Equal(t, org, orgID)
				require.Equal(t, user, userID)
				return tc.role, tc.repoErr
			}}
			auth, err := service.NewAuthService(repo, &hasherStub{}, &tokenStub{})
			require.NoError(t, err)
			err = auth.CheckOrganizerAccess(ctx, tc.org, tc.user)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.calls, calls)
		})
	}
}
