package session_service

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	"time"
)

func (s *SessionService) UpdateSession(ctx context.Context, input UpdateSessionInput, eventID, sessionID, organizerID, userID uuid.UUID) (domain.Session, error) {
	if input.StartsAt == nil && !input.EndsAtSet {
		return domain.Session{}, fmt.Errorf("empty session update: %w", coreerrors.ErrInvalidArgument)
	}
	if input.EndsAt != nil && !input.EndsAtSet {
		return domain.Session{}, fmt.Errorf("end time presence is required: %w", coreerrors.ErrInvalidArgument)
	}
	expected, err := s.GetSession(ctx, eventID, sessionID, organizerID, userID)
	if err != nil {
		return domain.Session{}, fmt.Errorf("get session: %w", err)
	}
	if expected.Status != domain.SessionDraft {
		return domain.Session{}, fmt.Errorf("session must be draft: %w", coreerrors.ErrConflict)
	}
	updated := expected
	if input.StartsAt != nil {
		updated.StartsAt = *input.StartsAt
	}
	if input.EndsAtSet {
		updated.EndsAt = input.EndsAt
	}
	if err := updated.Validate(); err != nil {
		return domain.Session{}, fmt.Errorf("validate session: %w", err)
	}
	if !updated.StartsAt.After(time.Now()) {
		return domain.Session{}, fmt.Errorf("session start must be in the future: %w", coreerrors.ErrInvalidArgument)
	}
	saved, err := s.sessionRepo.UpdateSession(ctx, updated, expected, organizerID, userID)
	if err != nil {
		return domain.Session{}, fmt.Errorf("update session: %w", err)
	}
	return saved, nil
}
