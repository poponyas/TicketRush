package auth_http_transport_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	coreauth "github.com/poponyas/TicketRush/internal/core/auth"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	password "github.com/poponyas/TicketRush/internal/core/security/password"
	token "github.com/poponyas/TicketRush/internal/core/security/token"
	repository "github.com/poponyas/TicketRush/internal/feature/identity/repository"
	service "github.com/poponyas/TicketRush/internal/feature/identity/service"
	transport "github.com/poponyas/TicketRush/internal/feature/identity/transport"
	"github.com/stretchr/testify/require"
)

func TestRegisterAndLoginPostgres(t *testing.T) {
	url := os.Getenv("TICKETRUSH_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TICKETRUSH_TEST_DATABASE_URL to a disposable database with migrations")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	defer pool.Close()
	email := uuid.New().String() + "@example.test"
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := pool.Exec(cleanupCtx, `DELETE FROM users WHERE email=$1`, email)
		if err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	}()
	hasher, err := password.NewHasher(password.DefaultConfig())
	require.NoError(t, err)
	manager, err := token.NewManager([]byte(strings.Repeat("s", 32)), "ticketrush", "ticketrush-api", 15*time.Minute)
	require.NoError(t, err)
	authRepo := repository.NewAuthRepo(&postgres.Pool{Pool: pool, OpTimeout: 5 * time.Second})
	auth, err := service.NewAuthService(authRepo, hasher, manager)
	require.NoError(t, err)
	h := transport.NewHTTPAuthTransport(auth)
	r := chi.NewRouter()
	r.Post("/api/v1/auth/register", h.Register)
	r.Post("/api/v1/auth/login", h.Login)
	r.With(coreauth.Authenticate(manager, auth)).Post("/api/v1/auth/logout", h.Logout)
	r.With(coreauth.Authenticate(manager, auth)).Get("/private", func(w http.ResponseWriter, r *http.Request) {
		identity, ok := coreauth.FromContext(r.Context())
		require.True(t, ok)
		require.NotEqual(t, uuid.Nil, identity.UserID)
		require.NotEqual(t, uuid.Nil, identity.SessionID)
		w.WriteHeader(http.StatusNoContent)
	})
	call := func(path, body, ct string, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", ct)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, status, w.Code, w.Body.String())
		return w
	}
	body := fmt.Sprintf(`{"email":%q,"password":"my safe test password"}`, email)
	call("/api/v1/auth/register", body, "application/json", 201)
	w := call("/api/v1/auth/login", body, "application/json; charset=utf-8", 200)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	var result transport.LoginResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.NotEmpty(t, result.RefreshToken)
	uid, sid, err := manager.VerifyAccess(result.AccessToken)
	require.NoError(t, err)
	require.Equal(t, result.ID, uid)
	var user uuid.UUID
	var storedHash string
	var expires time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT user_id,refresh_token_hash,expires_at FROM auth_sessions WHERE id=$1`, sid).Scan(&user, &storedHash, &expires))
	require.Equal(t, uid, user)
	require.True(t, expires.After(time.Now()))
	raw, err := base64.RawURLEncoding.DecodeString(result.RefreshToken)
	require.NoError(t, err)
	require.Len(t, raw, 32)
	digest := sha256.Sum256([]byte(result.RefreshToken))
	require.Equal(t, hex.EncodeToString(digest[:]), storedHash)
	w = call("/api/v1/auth/login", body, "application/json", 200)
	var again transport.LoginResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &again))
	require.NotEqual(t, result.RefreshToken, again.RefreshToken)
	_, secondSession, err := manager.VerifyAccess(again.AccessToken)
	require.NoError(t, err)
	require.NotEqual(t, sid, secondSession)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM auth_sessions WHERE user_id=$1`, uid).Scan(&count))
	require.Equal(t, 2, count)
	wrong := call("/api/v1/auth/login", fmt.Sprintf(`{"email":%q,"password":"wrong"}`, email), "application/json", 401)
	unknown := call("/api/v1/auth/login", `{"email":"unknown@example.test","password":"wrong"}`, "application/json", 401)
	require.Equal(t, wrong.Body.String(), unknown.Body.String())
	require.NotContains(t, wrong.Body.String(), "access_token")
	for _, bad := range []string{`{}`, `{"email":`, body + ` {}`, `{"email":"user@example.test","password":"x","role":"admin"}`, `{"email":"x","email":"y"}`} {
		call("/api/v1/auth/login", bad, "application/json", 400)
	}
	call("/api/v1/auth/login", body, "text/plain", 415)
	call("/api/v1/auth/login", `{"password":"`+strings.Repeat("x", 16<<10)+`"}`, "application/json", 413)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM auth_sessions WHERE user_id=$1`, uid).Scan(&count))
	require.Equal(t, 2, count)
	// Every protected request checks the persisted session, even for a valid JWT.
	private := func(access string, status int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/private", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, status, w.Code, w.Body.String())
	}
	private(result.AccessToken, 204)
	private(again.AccessToken, 204)
	unknownSession, err := manager.IssueAccess(uid, uuid.New())
	require.NoError(t, err)
	private(unknownSession, 401)
	wrongUser, err := manager.IssueAccess(uuid.New(), sid)
	require.NoError(t, err)
	private(wrongUser, 401)
	// A caller cannot revoke someone else's session by changing userID.
	require.NoError(t, authRepo.RevokeSession(ctx, sid, uuid.New()))
	private(result.AccessToken, 204)
	logout := func(access string, status int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		if access != "" {
			req.Header.Set("Authorization", "Bearer "+access)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, status, w.Code, w.Body.String())
		if status == 204 {
			require.Empty(t, w.Body.String())
		}
	}
	logout("", 401)
	logout("invalid-token", 401)
	// No organizer role or X-Organizer-ID is needed to log out.
	logout(result.AccessToken, 204)
	var revokedAt time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT revoked_at FROM auth_sessions WHERE id=$1`, sid).Scan(&revokedAt))
	require.False(t, revokedAt.IsZero())
	// Repeating the repository operation preserves the original revocation time.
	require.NoError(t, authRepo.RevokeSession(ctx, sid, uid))
	var againRevokedAt time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT revoked_at FROM auth_sessions WHERE id=$1`, sid).Scan(&againRevokedAt))
	require.Equal(t, revokedAt, againRevokedAt)
	private(result.AccessToken, 401)
	logout(result.AccessToken, 401)
	private(again.AccessToken, 204)
	_, err = pool.Exec(ctx, `UPDATE auth_sessions SET expires_at=NOW()-INTERVAL '1 second' WHERE id=$1`, secondSession)
	require.NoError(t, err)
	private(again.AccessToken, 401)
	_, err = pool.Exec(ctx, `DELETE FROM auth_sessions WHERE id=$1`, sid)
	require.NoError(t, err)
	private(result.AccessToken, 401)

}
