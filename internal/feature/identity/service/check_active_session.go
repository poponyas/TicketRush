package auth_service

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *AuthService) CheckActiveSession(ctx context.Context, sessionID, userID uuid.UUID) error {
	if sessionID == uuid.Nil || userID == uuid.Nil {
		return coreerrors.ErrUnauthenticated
	}
	if _, err := s.authRepo.GetActiveSession(ctx, sessionID, userID); err != nil {
		if errors.Is(err, coreerrors.ErrNotFound) {
			return coreerrors.ErrUnauthenticated
		}
		return fmt.Errorf("check active auth session: %w", err)
	}
	return nil
}
