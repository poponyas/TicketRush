package auth_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *AuthService) Logout(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) error {
	if sessionID == uuid.Nil || userID == uuid.Nil {
		return fmt.Errorf("session and user IDs are required: %w", core_errors.ErrInvalidArgument)
	}

	if err := s.authRepo.RevokeSession(ctx, sessionID, userID); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	return nil
}
