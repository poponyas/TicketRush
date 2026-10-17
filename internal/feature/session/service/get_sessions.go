package session_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *SessionService) GetSessions(ctx context.Context, eventID, organizerID, userID uuid.UUID, limit, offset *int) (SessionsPage, error) {
	if eventID == uuid.Nil || organizerID == uuid.Nil || userID == uuid.Nil {
		return SessionsPage{}, fmt.Errorf("event, organizer and user IDs are required: %w", coreerrors.ErrInvalidArgument)
	}
	page := SessionsPage{Limit: 20}
	if limit != nil {
		page.Limit = *limit
	}
	if offset != nil {
		page.Offset = *offset
	}
	if page.Limit < 1 || page.Limit > 100 {
		return SessionsPage{}, fmt.Errorf("limit must be between 1 and 100: %w", coreerrors.ErrInvalidArgument)
	}
	if page.Offset < 0 {
		return SessionsPage{}, fmt.Errorf("offset must be nonnegative: %w", coreerrors.ErrInvalidArgument)
	}
	sessions, err := s.sessionRepo.GetSessions(ctx, eventID, organizerID, userID, page.Limit, page.Offset)
	if err != nil {
		return SessionsPage{}, fmt.Errorf("list sessions: %w", err)
	}
	page.Sessions = sessions
	return page, nil
}
