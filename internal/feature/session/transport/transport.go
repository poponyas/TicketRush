package session_transport

import (
	"context"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	service "github.com/poponyas/TicketRush/internal/feature/session/service"
)

type HTTPSessionTransport struct {
	sessionService SessionService
}

type SessionService interface {
	CreateSession(ctx context.Context,
		request service.CreateSessionInput,
		organizerID uuid.UUID,
		userID uuid.UUID,
		eventID uuid.UUID,
	) (domain.Session, error)
	GetSession(ctx context.Context, eventID, sessionID, organizerID, userID uuid.UUID) (domain.Session, error)
	GetSessions(ctx context.Context, eventID, organizerID, userID uuid.UUID, limit, offset *int) (service.SessionsPage, error)
	UpdateSession(ctx context.Context, input service.UpdateSessionInput, eventID, sessionID, organizerID, userID uuid.UUID) (domain.Session, error)
	PublishSession(
		ctx context.Context,
		eventID, sessionID, organizerID, userID uuid.UUID,
	) (domain.Session, error)
}

func NewHTTPSessionTransport(
	sessionService SessionService,
) *HTTPSessionTransport {
	return &HTTPSessionTransport{
		sessionService: sessionService,
	}
}
