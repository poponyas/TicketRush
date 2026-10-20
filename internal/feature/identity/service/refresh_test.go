package auth_service_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/identity/service"
	"github.com/stretchr/testify/require"
)

func TestRefresh(t *testing.T) {
	ctx := context.Background()
	sid, uid := uuid.New(), uuid.New()
	oldToken := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	oldDigest := sha256.Sum256([]byte(oldToken))
	oldHash := hex.EncodeToString(oldDigest[:])
	var rotatedHash string
	var steps []string
	repo := &repoStub{
		refreshSession: func(got context.Context, hash string) (domain.AuthSession, error) {
			steps = append(steps, "lookup")
			require.Equal(t, ctx, got)
			require.Equal(t, oldHash, hash)
			return domain.AuthSession{ID: sid, UserID: uid}, nil
		},
		rotate: func(got context.Context, sessionID, userID uuid.UUID, old, new string) error {
			steps = append(steps, "rotate")
			require.Equal(t, ctx, got)
			require.Equal(t, sid, sessionID)
			require.Equal(t, uid, userID)
			require.Equal(t, oldHash, old)
			require.NotEqual(t, old, new)
			rotatedHash = new
			return nil
		},
	}
	tokens := &tokenStub{issue: func(userID, sessionID uuid.UUID) (string, error) {
		steps = append(steps, "sign")
		require.Equal(t, uid, userID)
		require.Equal(t, sid, sessionID)
		return "new-access", nil
	}}
	auth, err := service.NewAuthService(repo, &hasherStub{}, tokens)
	require.NoError(t, err)
	result, err := auth.Refresh(ctx, service.RefreshInput{RefreshToken: oldToken})
	require.NoError(t, err)
	require.Equal(t, []string{"lookup", "sign", "rotate"}, steps)
	require.Equal(t, "new-access", result.AccessToken)
	require.NotEqual(t, oldToken, result.RefreshToken)
	raw, err := base64.RawURLEncoding.DecodeString(result.RefreshToken)
	require.NoError(t, err)
	require.Len(t, raw, 32)
	digest := sha256.Sum256([]byte(result.RefreshToken))
	require.Equal(t, hex.EncodeToString(digest[:]), rotatedHash)
}

func TestRefreshRejectsInvalidTokenFormat(t *testing.T) {
	canonical := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	for _, raw := range []string{"", "invalid", strings.Repeat("x", 10000), canonical + "=", canonical + "\n", strings.Repeat("!", 43), base64.RawURLEncoding.EncodeToString(make([]byte, 31)), canonical[:42] + "B"} {
		auth, err := service.NewAuthService(&repoStub{}, &hasherStub{}, &tokenStub{})
		require.NoError(t, err)
		result, err := auth.Refresh(context.Background(), service.RefreshInput{RefreshToken: raw})
		require.ErrorIs(t, err, coreerrors.ErrUnauthenticated)
		require.Equal(t, service.RefreshResult{}, result)
	}
}

func TestRefreshFailuresReturnNoTokens(t *testing.T) {
	failure := errors.New("dependency failed")
	for _, tc := range []struct {
		name                                 string
		lookupErr, issueErr, rotateErr, want error
		issues, rotations                    int
	}{
		{name: "missing expired or revoked", lookupErr: fmt.Errorf("lookup: %w", coreerrors.ErrNotFound), want: coreerrors.ErrUnauthenticated},
		{name: "lookup failure", lookupErr: failure, want: failure},
		{name: "signing failure", issueErr: failure, want: failure, issues: 1},
		{name: "concurrent refresh or logout", rotateErr: fmt.Errorf("rotate: %w", coreerrors.ErrNotFound), want: coreerrors.ErrUnauthenticated, issues: 1, rotations: 1},
		{name: "write failure", rotateErr: failure, want: failure, issues: 1, rotations: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issues, rotations := 0, 0
			repo := &repoStub{
				refreshSession: func(context.Context, string) (domain.AuthSession, error) {
					return domain.AuthSession{ID: uuid.New(), UserID: uuid.New()}, tc.lookupErr
				},
				rotate: func(context.Context, uuid.UUID, uuid.UUID, string, string) error { rotations++; return tc.rotateErr },
			}
			tokens := &tokenStub{issue: func(uuid.UUID, uuid.UUID) (string, error) { issues++; return "access", tc.issueErr }}
			auth, err := service.NewAuthService(repo, &hasherStub{}, tokens)
			require.NoError(t, err)
			result, err := auth.Refresh(context.Background(), service.RefreshInput{RefreshToken: base64.RawURLEncoding.EncodeToString(make([]byte, 32))})
			require.ErrorIs(t, err, tc.want)
			require.Equal(t, service.RefreshResult{}, result)
			require.Equal(t, tc.issues, issues)
			require.Equal(t, tc.rotations, rotations)
		})
	}
}
