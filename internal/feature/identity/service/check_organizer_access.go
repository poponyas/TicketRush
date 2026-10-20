package auth_service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *AuthService) CheckOrganizerAccess(ctx context.Context, organizerID, userID uuid.UUID) error {
	if organizerID == uuid.Nil || userID == uuid.Nil {
		return coreerrors.ErrInvalidArgument
	}
	role, err := s.authRepo.GetOrganizerRole(ctx, organizerID, userID)
	if err != nil {
		if errors.Is(err, coreerrors.ErrNotFound) {
			return coreerrors.ErrForbidden
		}
		return fmt.Errorf("check organizer access: %w", err)
	}
	if role != domain.RoleOrganizer {
		return coreerrors.ErrForbidden
	}
	return nil
}
