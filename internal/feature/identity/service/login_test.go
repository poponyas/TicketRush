package auth_service_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	repository "github.com/poponyas/TicketRush/internal/feature/identity/repository"
	service "github.com/poponyas/TicketRush/internal/feature/identity/service"
	"github.com/stretchr/testify/require"
)

type repoStub struct {
	refreshSession func(context.Context, string) (domain.AuthSession, error)
	rotate         func(context.Context, uuid.UUID, uuid.UUID, string, string) error
	revoke         func(context.Context, uuid.UUID, uuid.UUID) error
	role           func(context.Context, uuid.UUID, uuid.UUID) (domain.Role, error)
	active         func(context.Context, uuid.UUID, uuid.UUID) (domain.AuthSession, error)
	get            func(context.Context, string) (repository.Credentials, error)
	create         func(context.Context, uuid.UUID, uuid.UUID, string, time.Time) error
}

func (r *repoStub) Register(context.Context, uuid.UUID, string, string) (uuid.UUID, string, error) {
	panic("unexpected register")
}
func (r *repoStub) GetCredentialsByEmail(ctx context.Context, email string) (repository.Credentials, error) {
	return r.get(ctx, email)
}
func (r *repoStub) CreateSession(ctx context.Context, sid, uid uuid.UUID, hash string, expires time.Time) error {
	return r.create(ctx, sid, uid, hash, expires)
}

type hasherStub struct {
	verify  func(string, string) (bool, error)
	hashErr error
}

func (h *hasherStub) Hash(string) (string, error)                { return "dummy-hash", h.hashErr }
func (h *hasherStub) Verify(password, hash string) (bool, error) { return h.verify(password, hash) }

type tokenStub struct {
	issue func(uuid.UUID, uuid.UUID) (string, error)
}

func (t *tokenStub) IssueAccess(uid, sid uuid.UUID) (string, error) { return t.issue(uid, sid) }

func TestLoginCreatesSession(t *testing.T) {
	uid := uuid.New()
	var issuedSession, storedSession uuid.UUID
	var storedHash string
	before := time.Now().UTC()
	repo := &repoStub{
		get: func(_ context.Context, email string) (repository.Credentials, error) {
			require.Equal(t, "user@example.com", email)
			return repository.Credentials{ID: uid, Email: email, PasswordHash: "stored-hash"}, nil
		},
		create: func(_ context.Context, sid, user uuid.UUID, hash string, expires time.Time) error {
			require.Equal(t, uid, user)
			storedSession = sid
			storedHash = hash
			require.False(t, expires.Before(before.Add(30*24*time.Hour)))
			require.False(t, expires.After(time.Now().UTC().Add(30*24*time.Hour)))
			return nil
		},
	}
	hasher := &hasherStub{verify: func(p, h string) (bool, error) {
		require.Equal(t, " my secret password ", p)
		require.Equal(t, "stored-hash", h)
		return true, nil
	}}
	tokens := &tokenStub{issue: func(user, sid uuid.UUID) (string, error) {
		require.Equal(t, uid, user)
		issuedSession = sid
		return "signed-access", nil
	}}
	auth, err := service.NewAuthService(repo, hasher, tokens)
	require.NoError(t, err)
	result, err := auth.Login(context.Background(), service.LoginInput{Email: " USER@example.com ", Password: " my secret password "})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, issuedSession)
	require.Equal(t, issuedSession, storedSession)
	require.Equal(t, uid, result.ID)
	require.Equal(t, "user@example.com", result.Email)
	require.Equal(t, "signed-access", result.AccessToken)
	raw, err := base64.RawURLEncoding.DecodeString(result.RefreshToken)
	require.NoError(t, err)
	require.Len(t, raw, 32)
	digest := sha256.Sum256([]byte(result.RefreshToken))
	require.Equal(t, hex.EncodeToString(digest[:]), storedHash)
	require.NotEqual(t, result.RefreshToken, storedHash)
}

func TestLoginFailuresDoNotReturnTokens(t *testing.T) {
	storageErr := errors.New("database unavailable")
	hashErr := errors.New("invalid encoded hash")
	signErr := errors.New("signing failed")
	for _, tc := range []struct {
		name                                       string
		getErr, verifyErr, issueErr, saveErr, want error
		verified                                   bool
		verifyCalls, issueCalls, saveCalls         int
	}{
		{name: "unknown user", getErr: coreerrors.ErrNotFound, want: coreerrors.ErrUnauthenticated, verifyCalls: 1},
		{name: "wrong password", want: coreerrors.ErrUnauthenticated, verifyCalls: 1},
		{name: "database read error", getErr: storageErr, want: storageErr},
		{name: "damaged hash", verifyErr: hashErr, want: hashErr, verifyCalls: 1},
		{name: "damaged dummy hash", getErr: coreerrors.ErrNotFound, verifyErr: hashErr, want: hashErr, verifyCalls: 1},
		{name: "signing error", verified: true, issueErr: signErr, want: signErr, verifyCalls: 1, issueCalls: 1},
		{name: "session insert error", verified: true, saveErr: storageErr, want: storageErr, verifyCalls: 1, issueCalls: 1, saveCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verifyCalls, issueCalls, saveCalls := 0, 0, 0
			repo := &repoStub{
				get: func(context.Context, string) (repository.Credentials, error) {
					return repository.Credentials{ID: uuid.New(), PasswordHash: "stored-hash"}, tc.getErr
				},
				create: func(context.Context, uuid.UUID, uuid.UUID, string, time.Time) error { saveCalls++; return tc.saveErr },
			}
			hasher := &hasherStub{verify: func(p, h string) (bool, error) {
				verifyCalls++
				require.Equal(t, "password", p)
				if tc.getErr == coreerrors.ErrNotFound {
					require.Equal(t, "dummy-hash", h)
				} else {
					require.Equal(t, "stored-hash", h)
				}
				return tc.verified, tc.verifyErr
			}}
			tokens := &tokenStub{issue: func(uuid.UUID, uuid.UUID) (string, error) { issueCalls++; return "access", tc.issueErr }}
			auth, err := service.NewAuthService(repo, hasher, tokens)
			require.NoError(t, err)
			result, err := auth.Login(context.Background(), service.LoginInput{Email: "user@example.com", Password: "password"})
			require.ErrorIs(t, err, tc.want)
			require.Equal(t, service.LoginResult{}, result)
			require.Equal(t, tc.verifyCalls, verifyCalls)
			require.Equal(t, tc.issueCalls, issueCalls)
			require.Equal(t, tc.saveCalls, saveCalls)
		})
	}
}

func TestAuthServiceReportsDummyHashFailure(t *testing.T) {
	want := errors.New("hash failed")
	auth, err := service.NewAuthService(&repoStub{}, &hasherStub{hashErr: want}, &tokenStub{})
	require.Nil(t, auth)
	require.ErrorIs(t, err, want)
}

func (r *repoStub) GetActiveSession(ctx context.Context, sid, uid uuid.UUID) (domain.AuthSession, error) {
	return r.active(ctx, sid, uid)
}

func (r *repoStub) GetOrganizerRole(ctx context.Context, org, user uuid.UUID) (domain.Role, error) {
	return r.role(ctx, org, user)
}

func (r *repoStub) RevokeSession(ctx context.Context, sid, uid uuid.UUID) error {
	return r.revoke(ctx, sid, uid)
}

func (r *repoStub) GetActiveSessionByRefreshHash(ctx context.Context, hash string) (domain.AuthSession, error) {
	return r.refreshSession(ctx, hash)
}
func (r *repoStub) RotateRefreshToken(ctx context.Context, sid, uid uuid.UUID, oldHash, newHash string) error {
	return r.rotate(ctx, sid, uid, oldHash, newHash)
}
