package session_service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *SessionService) CreateSession(ctx context.Context,
	request CreateSessionInput,
	organizerID uuid.UUID,
	userID uuid.UUID,
	eventID uuid.UUID,
) (domain.Session, error) {
	if userID == uuid.Nil || organizerID == uuid.Nil || eventID == uuid.Nil {
		return domain.Session{}, fmt.Errorf("event, organizer and user IDs are required: %w", coreerrors.ErrInvalidArgument)
	}
	session := domain.NewSession(
		eventID,
		request.VenueID,
		request.StartsAt,
		request.EndsAt,
		request.SalesStartAt,
		request.SalesEndAt,
	)
	// Use one server-side instant for creation time and current-time checks.
	now := time.Now().UTC()
	session.CreatedAt = now

	if err := session.Validate(); err != nil {
		return domain.Session{}, fmt.Errorf("validate error: %w", err)
	}
	if !session.StartsAt.After(now) || !session.SalesEndAt.After(now) {
		return domain.Session{}, fmt.Errorf("session start and sales end must be in the future: %w", coreerrors.ErrInvalidArgument)
	}

	response, err := s.sessionRepo.CreateSession(ctx, session, organizerID, userID)
	if err != nil {
		return domain.Session{}, fmt.Errorf("create session: %w", err)
	}

	return response, nil
}
