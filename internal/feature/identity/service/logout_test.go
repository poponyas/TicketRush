package auth_service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/identity/service"
	"github.com/stretchr/testify/require"
)

func TestLogout(t *testing.T) {
	session, user := uuid.New(), uuid.New()
	databaseErr := errors.New("database unavailable")
	for _, tc := range []struct {
		name          string
		session, user uuid.UUID
		repoErr, want error
		calls         int
	}{
		{name: "success", session: session, user: user, calls: 1},
		{name: "database error", session: session, user: user, repoErr: databaseErr, want: databaseErr, calls: 1},
		{name: "cancelled", session: session, user: user, repoErr: context.Canceled, want: context.Canceled, calls: 1},
		{name: "nil session", user: user, want: coreerrors.ErrInvalidArgument},
		{name: "nil user", session: session, want: coreerrors.ErrInvalidArgument},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			ctx := context.Background()
			repo := &repoStub{revoke: func(got context.Context, sid, uid uuid.UUID) error {
				calls++
				require.Equal(t, ctx, got)
				require.Equal(t, session, sid)
				require.Equal(t, user, uid)
				return tc.repoErr
			}}
			auth, err := service.NewAuthService(repo, &hasherStub{}, &tokenStub{})
			require.NoError(t, err)
			err = auth.Logout(ctx, tc.session, tc.user)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.calls, calls)
		})
	}
}
