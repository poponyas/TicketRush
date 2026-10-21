package session_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *SessionService) GetPublicSession(ctx context.Context, eventID, sessionID uuid.UUID) (domain.Session, error) {
	if eventID == uuid.Nil || sessionID == uuid.Nil {
		return domain.Session{}, fmt.Errorf("event and session IDs are required: %w", coreerrors.ErrInvalidArgument)
	}
	session, err := s.sessionRepo.GetPublicSession(ctx, eventID, sessionID)
	if err != nil {
		return domain.Session{}, fmt.Errorf("get public session: %w", err)
	}
	return session, nil
}

func (s *SessionService) GetPublicSessions(ctx context.Context, eventID uuid.UUID, limit, offset *int) (SessionsPage, error) {
	if eventID == uuid.Nil {
		return SessionsPage{}, fmt.Errorf("event ID is required: %w", coreerrors.ErrInvalidArgument)
	}
	page := SessionsPage{Limit: 20}
	if limit != nil {
		page.Limit = *limit
	}
	if offset != nil {
		page.Offset = *offset
	}
	if page.Limit < 1 || page.Limit > 100 || page.Offset < 0 {
		return SessionsPage{}, fmt.Errorf("limit must be 1..100 and offset nonnegative: %w", coreerrors.ErrInvalidArgument)
	}
	sessions, err := s.sessionRepo.GetPublicSessions(ctx, eventID, page.Limit, page.Offset)
	if err != nil {
		return SessionsPage{}, fmt.Errorf("list public sessions: %w", err)
	}
	page.Sessions = sessions
	return page, nil
}
