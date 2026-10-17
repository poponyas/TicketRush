package session_service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/session/service"
	"github.com/stretchr/testify/require"
)

func TestUpdateSessionService(t *testing.T) {
	now := time.Now().UTC()
	end := now.Add(72 * time.Hour)
	original := domain.NewSession(uuid.New(), uuid.New(), now.Add(48*time.Hour), &end, now.Add(-time.Hour), now.Add(24*time.Hour))
	original.CreatedAt = now
	org, user := uuid.New(), uuid.New()
	start := now.Add(49 * time.Hour)
	zero := time.Time{}
	past := now.Add(-time.Hour)
	badEnd := original.StartsAt.Add(-time.Hour)
	for _, tc := range []struct {
		name   string
		input  service.UpdateSessionInput
		status domain.SessionStatus
		want   error
	}{
		{"start only", service.UpdateSessionInput{StartsAt: &start}, domain.SessionDraft, nil},
		{"clear end", service.UpdateSessionInput{EndsAtSet: true}, domain.SessionDraft, nil},
		{"empty", service.UpdateSessionInput{}, domain.SessionDraft, coreerrors.ErrInvalidArgument},
		{"zero start", service.UpdateSessionInput{StartsAt: &zero}, domain.SessionDraft, coreerrors.ErrInvalidArgument},
		{"zero end", service.UpdateSessionInput{EndsAt: &zero, EndsAtSet: true}, domain.SessionDraft, coreerrors.ErrInvalidArgument},
		{"end before start", service.UpdateSessionInput{EndsAt: &badEnd, EndsAtSet: true}, domain.SessionDraft, coreerrors.ErrInvalidArgument},
		{"past start", service.UpdateSessionInput{StartsAt: &past}, domain.SessionDraft, coreerrors.ErrInvalidArgument},
		{"scheduled", service.UpdateSessionInput{StartsAt: &start}, domain.SessionScheduled, coreerrors.ErrConflict},
		{"cancelled", service.UpdateSessionInput{StartsAt: &start}, domain.SessionCancelled, coreerrors.ErrConflict},
		{"completed", service.UpdateSessionInput{StartsAt: &start}, domain.SessionCompleted, coreerrors.ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stored := original
			stored.Status = tc.status
			called := false
			repo := &sessionRepoStub{
				get: func(_ context.Context, eventID, sessionID, orgID, userID uuid.UUID) (domain.Session, error) {
					require.Equal(t, stored.EventID, eventID)
					require.Equal(t, stored.ID, sessionID)
					require.Equal(t, org, orgID)
					require.Equal(t, user, userID)
					return stored, nil
				},
				update: func(_ context.Context, updated, expected domain.Session, orgID, userID uuid.UUID) (domain.Session, error) {
					called = true
					require.Equal(t, stored, expected)
					require.Equal(t, org, orgID)
					require.Equal(t, user, userID)
					if tc.input.StartsAt != nil {
						require.Equal(t, *tc.input.StartsAt, updated.StartsAt)
					} else {
						require.Equal(t, stored.StartsAt, updated.StartsAt)
					}
					if tc.input.EndsAtSet {
						require.Equal(t, tc.input.EndsAt, updated.EndsAt)
					} else {
						require.Equal(t, stored.EndsAt, updated.EndsAt)
					}
					require.Equal(t, stored.SalesEndAt, updated.SalesEndAt)
					require.Equal(t, stored.CreatedAt, updated.CreatedAt)
					return updated, nil
				},
			}
			_, err := service.NewSessionService(repo).UpdateSession(context.Background(), tc.input, stored.EventID, stored.ID, org, user)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
				require.False(t, called)
			} else {
				require.NoError(t, err)
				require.True(t, called)
			}
		})
	}
	// Invariant-valid historical sessions still cannot be moved into the past.
	historical := original
	historical.SalesStartAt = now.Add(-5 * time.Hour)
	historical.SalesEndAt = now.Add(-3 * time.Hour)
	repo := &sessionRepoStub{get: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Session, error) {
		return historical, nil
	}}
	_, err := service.NewSessionService(repo).UpdateSession(context.Background(), service.UpdateSessionInput{StartsAt: &past}, original.EventID, original.ID, org, user)
	require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	for _, want := range []error{coreerrors.ErrNotFound, coreerrors.ErrConflict} {
		repo.get = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Session, error) {
			return original, nil
		}
		repo.update = func(context.Context, domain.Session, domain.Session, uuid.UUID, uuid.UUID) (domain.Session, error) {
			return domain.Session{}, fmt.Errorf("storage: %w", want)
		}
		_, err = service.NewSessionService(repo).UpdateSession(context.Background(), service.UpdateSessionInput{StartsAt: &start}, original.EventID, original.ID, org, user)
		require.ErrorIs(t, err, want)
	}
}
