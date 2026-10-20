package auth_http_transport_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
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

func TestRefreshPostgres(t *testing.T) {
	url := os.Getenv("TICKETRUSH_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TICKETRUSH_TEST_DATABASE_URL to a disposable database with migrations")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	defer pool.Close()
	user := uuid.New()
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := pool.Exec(cleanup, `DELETE FROM users WHERE id=$1`, user)
		if err != nil {
			t.Errorf("cleanup refresh user: %v", err)
		}
	}()
	_, err = pool.Exec(ctx, `INSERT INTO users(id,email,password_hash) VALUES($1,$2,'unused-test-hash')`, user, user.String()+"@example.test")
	require.NoError(t, err)
	repo := repository.NewAuthRepo(&postgres.Pool{Pool: pool, OpTimeout: 5 * time.Second})
	hasher, err := password.NewHasher(password.DefaultConfig())
	require.NoError(t, err)
	manager, err := token.NewManager([]byte(strings.Repeat("s", 32)), "ticketrush", "ticketrush-api", 15*time.Minute)
	require.NoError(t, err)
	auth, err := service.NewAuthService(repo, hasher, manager)
	require.NoError(t, err)
	h := transport.NewHTTPAuthTransport(auth)
	router := chi.NewRouter()
	router.Post("/api/v1/auth/refresh", h.Refresh)
	router.With(coreauth.Authenticate(manager, auth)).Post("/api/v1/auth/logout", h.Logout)
	router.With(coreauth.Authenticate(manager, auth)).Get("/private", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	createSession := func(expires time.Time) (uuid.UUID, string) {
		t.Helper()
		raw := make([]byte, 32)
		_, err := rand.Read(raw)
		require.NoError(t, err)
		refresh := base64.RawURLEncoding.EncodeToString(raw)
		hash := sha256.Sum256([]byte(refresh))
		sid := uuid.New()
		require.NoError(t, repo.CreateSession(ctx, sid, user, hex.EncodeToString(hash[:]), expires))
		return sid, refresh
	}
	body := func(raw string) string {
		data, err := json.Marshal(transport.RefreshRequest{RefreshToken: raw})
		require.NoError(t, err)
		return string(data)
	}
	call := func(payload, ct string, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(payload))
		req.Header.Set("Content-Type", ct)
		// Refresh must not require a working access JWT.
		req.Header.Set("Authorization", "Bearer expired-or-invalid-access")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, status, w.Code, w.Body.String())
		require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		return w
	}
	private := func(access string, status int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/private", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, status, w.Code, w.Body.String())
	}
	expires := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	sid, original := createSession(expires)
	w := call(body(original), "application/json; charset=utf-8", 200)
	var first transport.RefreshResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &first))
	require.NotEqual(t, original, first.RefreshToken)
	uid, gotSession, err := manager.VerifyAccess(first.AccessToken)
	require.NoError(t, err)
	require.Equal(t, user, uid)
	require.Equal(t, sid, gotSession)
	private(first.AccessToken, 204)
	var storedHash string
	var storedExpiry time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT refresh_token_hash,expires_at FROM auth_sessions WHERE id=$1`, sid).Scan(&storedHash, &storedExpiry))
	digest := sha256.Sum256([]byte(first.RefreshToken))
	require.Equal(t, hex.EncodeToString(digest[:]), storedHash)
	require.True(t, expires.Equal(storedExpiry))
	call(body(original), "application/json", 401)
	w = call(body(first.RefreshToken), "application/json", 200)
	var second transport.RefreshResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &second))
	require.NotEqual(t, first.RefreshToken, second.RefreshToken)
	call(body(first.RefreshToken), "application/json", 401)
	for _, bad := range []string{`{"refresh_token":`, body(second.RefreshToken) + ` {}`, `{"refresh_token":"x","extra":true}`, `{"refresh_token":1}`, `{"refresh_token":"x","refresh_token":"y"}`} {
		call(bad, "application/json", 400)
	}
	for _, bad := range []string{`{}`, `null`, `{"refresh_token":null}`, `{"refresh_token":"invalid"}`} {
		call(bad, "application/json", 401)
	}
	call(body(second.RefreshToken), "text/plain", 415)
	call(`{"refresh_token":"`+strings.Repeat("x", 16<<10)+`"}`, "application/json", 413)
	// Invalid requests must not consume the valid refresh token.
	w = call(body(second.RefreshToken), "application/json", 200)
	var current transport.RefreshResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &current))
	logout := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logout.Header.Set("Authorization", "Bearer "+current.AccessToken)
	logoutResponse := httptest.NewRecorder()
	router.ServeHTTP(logoutResponse, logout)
	require.Equal(t, 204, logoutResponse.Code)
	private(current.AccessToken, 401)
	call(body(current.RefreshToken), "application/json", 401)
	_, expired := createSession(time.Now().Add(-time.Hour))
	call(body(expired), "application/json", 401)
	unknown := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	call(body(unknown), "application/json", 401)
	// Only one request may consume the same old hash under contention.
	_, concurrent := createSession(expires)
	payload := body(concurrent)
	const requests = 6
	responses := make([]*httptest.ResponseRecorder, requests)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range requests {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			responses[i] = httptest.NewRecorder()
			router.ServeHTTP(responses[i], req)
		}(i)
	}
	close(start)
	wg.Wait()
	successes := 0
	var winner transport.RefreshResponse
	for _, response := range responses {
		if response.Code == 200 {
			successes++
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &winner))
		} else {
			require.Equal(t, 401, response.Code, response.Body.String())
			require.NotContains(t, response.Body.String(), "access_token")
		}
	}
	require.Equal(t, 1, successes, "one old refresh must have one winner")
	call(body(concurrent), "application/json", 401)
	call(body(winner.RefreshToken), "application/json", 200)
}
