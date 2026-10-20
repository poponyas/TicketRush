package auth_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	token "github.com/poponyas/TicketRush/internal/core/security/token"
	"github.com/stretchr/testify/require"
)

type verifierFunc func(string) (uuid.UUID, uuid.UUID, error)

func (f verifierFunc) VerifyAccess(raw string) (uuid.UUID, uuid.UUID, error) { return f(raw) }

func TestAuthenticateRejectsMalformedAuthorization(t *testing.T) {
	for _, values := range [][]string{
		nil, {""}, {"Bearer"}, {"Bearer "}, {"Basic secret"}, {"Bearer one two"},
		{"Bearer one", "Bearer two"}, {"Bearer one, Bearer two"},
	} {
		t.Run(strings.Join(values, "|"), func(t *testing.T) {
			verifier := verifierFunc(func(string) (uuid.UUID, uuid.UUID, error) {
				t.Fatal("malformed header must not reach the verifier")
				return uuid.Nil, uuid.Nil, nil
			})
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("handler must not run") })
			req := httptest.NewRequest(http.MethodGet, "/private", nil)
			for _, value := range values {
				req.Header.Add("Authorization", value)
			}
			w := httptest.NewRecorder()
			auth.Authenticate(verifier, allowSessions())(next).ServeHTTP(w, req)
			require.Equal(t, http.StatusUnauthorized, w.Code)
			require.Equal(t, "Bearer", w.Header().Get("WWW-Authenticate"))
			require.JSONEq(t, `{"code":"unauthenticated","message":"authentication required"}`, w.Body.String())
		})
	}
}

func TestAuthenticateStoresVerifiedIdentity(t *testing.T) {
	userID, sessionID := uuid.New(), uuid.New()
	type requestKey struct{}
	for _, header := range []string{"Bearer access", "bearer access", "BEARER   access"} {
		t.Run(header, func(t *testing.T) {
			called := false
			verifier := verifierFunc(func(raw string) (uuid.UUID, uuid.UUID, error) {
				require.Equal(t, "access", raw)
				return userID, sessionID, nil
			})
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				identity, ok := auth.FromContext(r.Context())
				require.True(t, ok)
				require.Equal(t, userID, identity.UserID)
				require.Equal(t, sessionID, identity.SessionID)
				require.Empty(t, identity.Roles)
				require.Equal(t, uuid.Nil, identity.OrganizerID)
				require.Equal(t, "kept", r.Context().Value(requestKey{}))
				w.WriteHeader(http.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodGet, "/private", nil)
			req.Header.Set("Authorization", header)
			ctx := context.WithValue(req.Context(), requestKey{}, "kept")
			// Authentication must not inherit privileges from an earlier identity.
			ctx = auth.WithIdentity(ctx, auth.Identity{UserID: uuid.New(), OrganizerID: uuid.New(), Roles: []domain.Role{domain.RoleOrganizer}})
			w := httptest.NewRecorder()
			auth.Authenticate(verifier, allowSessions())(next).ServeHTTP(w, req.WithContext(ctx))
			require.True(t, called)
			require.Equal(t, http.StatusNoContent, w.Code)
		})
	}
}

func TestAuthenticateRejectsInvalidVerifierResult(t *testing.T) {
	for _, tc := range []struct {
		user, session uuid.UUID
		err           error
	}{
		{uuid.New(), uuid.New(), errors.New("parser error with private-token")},
		{uuid.Nil, uuid.New(), nil}, {uuid.New(), uuid.Nil, nil},
	} {
		verifier := verifierFunc(func(string) (uuid.UUID, uuid.UUID, error) { return tc.user, tc.session, tc.err })
		req := httptest.NewRequest(http.MethodGet, "/private", nil)
		req.Header.Set("Authorization", "Bearer private-token")
		w := httptest.NewRecorder()
		auth.Authenticate(verifier, allowSessions())(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("handler must not run") })).ServeHTTP(w, req)
		require.Equal(t, http.StatusUnauthorized, w.Code)
		require.NotContains(t, w.Body.String(), "private-token")
	}
}

func TestAuthenticateWithJWTManager(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	manager, err := token.NewManager(secret, "ticketrush", "ticketrush-api", 15*time.Minute)
	require.NoError(t, err)
	userID, sessionID := uuid.New(), uuid.New()
	valid, err := manager.IssueAccess(userID, sessionID)
	require.NoError(t, err)
	sign := func(change func(*token.AccessClaims), method jwt.SigningMethod, key []byte) string {
		claims := token.AccessClaims{SessionID: sessionID, RegisteredClaims: jwt.RegisteredClaims{
			Subject: userID.String(), Issuer: "ticketrush", Audience: jwt.ClaimStrings{"ticketrush-api"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}}
		change(&claims)
		raw, err := jwt.NewWithClaims(method, claims).SignedString(key)
		require.NoError(t, err)
		return raw
	}
	noop := func(*token.AccessClaims) {}
	for _, tc := range []struct {
		name, raw string
		status    int
	}{
		{"valid", valid, 204},
		{"expired", sign(func(c *token.AccessClaims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour)) }, jwt.SigningMethodHS256, secret), 401},
		{"missing expiry", sign(func(c *token.AccessClaims) { c.ExpiresAt = nil }, jwt.SigningMethodHS256, secret), 401},
		{"wrong issuer", sign(func(c *token.AccessClaims) { c.Issuer = "foreign" }, jwt.SigningMethodHS256, secret), 401},
		{"wrong audience", sign(func(c *token.AccessClaims) { c.Audience = jwt.ClaimStrings{"foreign"} }, jwt.SigningMethodHS256, secret), 401},
		{"wrong key", sign(noop, jwt.SigningMethodHS256, []byte(strings.Repeat("x", 32))), 401},
		{"wrong algorithm", sign(noop, jwt.SigningMethodHS384, secret), 401},
		{"invalid subject", sign(func(c *token.AccessClaims) { c.Subject = "bad" }, jwt.SigningMethodHS256, secret), 401},
		{"missing session", sign(func(c *token.AccessClaims) { c.SessionID = uuid.Nil }, jwt.SigningMethodHS256, secret), 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				identity, ok := auth.FromContext(r.Context())
				require.True(t, ok)
				require.Equal(t, userID, identity.UserID)
				require.Equal(t, sessionID, identity.SessionID)
				w.WriteHeader(http.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodGet, "/private", nil)
			req.Header.Set("Authorization", "Bearer "+tc.raw)
			w := httptest.NewRecorder()
			auth.Authenticate(manager, allowSessions())(next).ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code)
		})
	}
}

type sessionCheckerFunc func(context.Context, uuid.UUID, uuid.UUID) error

func (f sessionCheckerFunc) CheckActiveSession(ctx context.Context, sid, uid uuid.UUID) error {
	return f(ctx, sid, uid)
}
func allowSessions() sessionCheckerFunc {
	return func(context.Context, uuid.UUID, uuid.UUID) error { return nil }
}

func TestAuthenticateChecksActiveSession(t *testing.T) {
	userID, sessionID := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"active", nil, 204},
		{"revoked or expired", fmt.Errorf("check: %w", coreerrors.ErrUnauthenticated), 401},
		{"database failure", errors.New("database unavailable"), 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			checked := false
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			verifier := verifierFunc(func(string) (uuid.UUID, uuid.UUID, error) { return userID, sessionID, nil })
			checker := sessionCheckerFunc(func(got context.Context, sid, uid uuid.UUID) error {
				checked = true
				require.Equal(t, ctx, got)
				require.Equal(t, sessionID, sid)
				require.Equal(t, userID, uid)
				return tc.err
			})
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(204) })
			req := httptest.NewRequest(http.MethodGet, "/private", nil).WithContext(ctx)
			req.Header.Set("Authorization", "Bearer access")
			w := httptest.NewRecorder()
			auth.Authenticate(verifier, checker)(next).ServeHTTP(w, req)
			require.True(t, checked)
			require.Equal(t, tc.status == 204, called)
			require.Equal(t, tc.status, w.Code)
			if tc.status == 500 {
				require.Empty(t, w.Header().Get("WWW-Authenticate"))
				require.NotContains(t, w.Body.String(), "database unavailable")
			}
		})
	}
}

func TestAuthenticateDoesNotQuerySessionsForInvalidToken(t *testing.T) {
	verifier := verifierFunc(func(string) (uuid.UUID, uuid.UUID, error) { return uuid.Nil, uuid.Nil, errors.New("bad signature") })
	checker := sessionCheckerFunc(func(context.Context, uuid.UUID, uuid.UUID) error {
		t.Fatal("invalid JWT must not query sessions")
		return nil
	})
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	w := httptest.NewRecorder()
	auth.Authenticate(verifier, checker)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("handler must not run") })).ServeHTTP(w, req)
	require.Equal(t, 401, w.Code)
}
