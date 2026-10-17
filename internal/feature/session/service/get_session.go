package session_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *SessionService) GetSession(ctx context.Context, eventID, sessionID, organizerID, userID uuid.UUID) (domain.Session, error) {
	if eventID == uuid.Nil || sessionID == uuid.Nil || organizerID == uuid.Nil || userID == uuid.Nil {
		return domain.Session{}, fmt.Errorf("event, session, organizer and user IDs are required: %w", coreerrors.ErrInvalidArgument)
	}
	session, err := s.sessionRepo.GetSession(ctx, eventID, sessionID, organizerID, userID)
	if err != nil {
		return domain.Session{}, fmt.Errorf("get session: %w", err)
	}
	return session, nil
}
