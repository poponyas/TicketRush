package session_service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
)

type SessionService struct {
	sessionRepo SessionRepository
}

type CreateSessionInput struct {
	VenueID      uuid.UUID
	StartsAt     time.Time
	EndsAt       *time.Time
	SalesStartAt time.Time
	SalesEndAt   time.Time
}

type UpdateSessionInput struct {
	StartsAt  *time.Time
	EndsAt    *time.Time
	EndsAtSet bool
}

type SessionRepository interface {
	CreateSession(ctx context.Context,
		session domain.Session,
		organizerID uuid.UUID,
		userID uuid.UUID,
	) (domain.Session, error)
	GetSession(ctx context.Context, eventID, sessionID, organizerID, userID uuid.UUID) (domain.Session, error)
	GetSessions(ctx context.Context, eventID, organizerID, userID uuid.UUID, limit, offset int) ([]domain.Session, error)
	UpdateSession(ctx context.Context, session, expected domain.Session, organizerID uuid.UUID, userID uuid.UUID) (domain.Session, error)
	PublishSession(
		ctx context.Context,
		eventID, sessionID, organizerID, userID uuid.UUID,
	) (domain.Session, error)
}

type SessionsPage struct {
	Sessions []domain.Session
	Limit    int
	Offset   int
}

func NewSessionService(
	sessionRepo SessionRepository,
) *SessionService {
	return &SessionService{
		sessionRepo: sessionRepo,
	}
}
