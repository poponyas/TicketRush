package auth_http_transport_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	password "github.com/poponyas/TicketRush/internal/core/security/password"
	token "github.com/poponyas/TicketRush/internal/core/security/token"
	repository "github.com/poponyas/TicketRush/internal/feature/identity/repository"
	service "github.com/poponyas/TicketRush/internal/feature/identity/service"
	"github.com/stretchr/testify/require"
)

func TestOrganizerAccessPostgres(t *testing.T) {
	url := os.Getenv("TICKETRUSH_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TICKETRUSH_TEST_DATABASE_URL to a disposable database with migrations")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	defer pool.Close()
	user, org, otherOrg, sid := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, step := range []struct {
			query string
			args  []any
		}{
			{`DELETE FROM organizer_memberships WHERE organizer_id IN ($1,$2)`, []any{org, otherOrg}},
			{`DELETE FROM organizers WHERE id IN ($1,$2)`, []any{org, otherOrg}},
			{`DELETE FROM users WHERE id=$1`, []any{user}},
		} {
			if _, err := pool.Exec(cleanup, step.query, step.args...); err != nil {
				t.Errorf("cleanup organizer fixture: %v", err)
			}
		}
	}()
	exec := func(query string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, query, args...)
		require.NoError(t, err)
	}
	exec(`INSERT INTO users(id,email,password_hash) VALUES($1,$2,'unused-test-hash')`, user, user.String()+"@example.test")
	exec(`INSERT INTO organizers(id,name) VALUES($1,'First'),($2,'Second')`, org, otherOrg)
	exec(`INSERT INTO organizer_memberships(organizer_id,user_id,role) VALUES($1,$3,'organizer'),($2,$3,'controller')`, org, otherOrg, user)
	repo := repository.NewAuthRepo(&postgres.Pool{Pool: pool, OpTimeout: 5 * time.Second})
	require.NoError(t, repo.CreateSession(ctx, sid, user, uuid.New().String(), time.Now().Add(time.Hour)))
	role, err := repo.GetOrganizerRole(ctx, org, user)
	require.NoError(t, err)
	require.Equal(t, domain.RoleOrganizer, role)
	role, err = repo.GetOrganizerRole(ctx, otherOrg, user)
	require.NoError(t, err)
	require.Equal(t, domain.RoleController, role)
	_, err = repo.GetOrganizerRole(ctx, org, uuid.New())
	require.ErrorIs(t, err, coreerrors.ErrNotFound)
	manager, err := token.NewManager([]byte(strings.Repeat("s", 32)), "ticketrush", "ticketrush-api", 15*time.Minute)
	require.NoError(t, err)
	hasher, err := password.NewHasher(password.DefaultConfig())
	require.NoError(t, err)
	authService, err := service.NewAuthService(repo, hasher, manager)
	require.NoError(t, err)
	access, err := manager.IssueAccess(user, sid)
	require.NoError(t, err)
	router := chi.NewRouter()
	router.With(auth.Authenticate(manager, authService), auth.SelectOrganizer(authService), auth.RequireOrganizer).Get("/organizer", func(w http.ResponseWriter, r *http.Request) {
		identity, ok := auth.FromContext(r.Context())
		require.True(t, ok)
		require.Equal(t, user, identity.UserID)
		require.Equal(t, sid, identity.SessionID)
		require.True(t, identity.OrganizerID == org || identity.OrganizerID == otherOrg)
		require.True(t, identity.HasRole(domain.RoleOrganizer))
		w.WriteHeader(204)
	})
	call := func(selected string, authenticated bool, status int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/organizer", nil)
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+access)
		}
		if selected != "" {
			req.Header.Set("X-Organizer-ID", selected)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, status, w.Code, w.Body.String())
	}
	call(org.String(), false, 401)
	call("", true, 400)
	call("invalid", true, 400)
	call(uuid.Nil.String(), true, 400)
	call(org.String(), true, 204)
	call(otherOrg.String(), true, 403)
	call(uuid.New().String(), true, 403)
	// The same token can select a second organization after a trusted role change.
	exec(`UPDATE organizer_memberships SET role='organizer' WHERE organizer_id=$1 AND user_id=$2`, otherOrg, user)
	call(otherOrg.String(), true, 204)
	// Role changes and membership revocation take effect on the next request.
	exec(`UPDATE organizer_memberships SET role='controller' WHERE organizer_id=$1 AND user_id=$2`, org, user)
	call(org.String(), true, 403)
	exec(`DELETE FROM organizer_memberships WHERE organizer_id=$1 AND user_id=$2`, otherOrg, user)
	call(otherOrg.String(), true, 403)
	exec(`UPDATE auth_sessions SET revoked_at=NOW() WHERE id=$1`, sid)
	call(org.String(), true, 401)
}
