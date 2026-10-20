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

func TestCheckActiveSession(t *testing.T) {
	sid, uid := uuid.New(), uuid.New()
	databaseErr := errors.New("database unavailable")
	for _, tc := range []struct {
		name          string
		sid, uid      uuid.UUID
		repoErr, want error
		calls         int
	}{
		{name: "active", sid: sid, uid: uid, calls: 1},
		{name: "missing expired or revoked", sid: sid, uid: uid, repoErr: fmt.Errorf("storage: %w", coreerrors.ErrNotFound), want: coreerrors.ErrUnauthenticated, calls: 1},
		{name: "database error", sid: sid, uid: uid, repoErr: databaseErr, want: databaseErr, calls: 1},
		{name: "cancelled", sid: sid, uid: uid, repoErr: context.Canceled, want: context.Canceled, calls: 1},
		{name: "nil session", sid: uuid.Nil, uid: uid, want: coreerrors.ErrUnauthenticated},
		{name: "nil user", sid: sid, uid: uuid.Nil, want: coreerrors.ErrUnauthenticated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			calls := 0
			repo := &repoStub{active: func(got context.Context, sessionID, userID uuid.UUID) (domain.AuthSession, error) {
				calls++
				require.Equal(t, ctx, got)
				require.Equal(t, sid, sessionID)
				require.Equal(t, uid, userID)
				return domain.AuthSession{ID: sid, UserID: uid}, tc.repoErr
			}}
			auth, err := service.NewAuthService(repo, &hasherStub{}, &tokenStub{})
			require.NoError(t, err)
			err = auth.CheckActiveSession(ctx, tc.sid, tc.uid)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.calls, calls)
		})
	}
}
