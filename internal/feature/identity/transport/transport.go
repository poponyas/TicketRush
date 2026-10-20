package auth_http_transport

import (
	"context"

	"github.com/google/uuid"
	auth_service "github.com/poponyas/TicketRush/internal/feature/identity/service"
)

type HTTPAuthTransport struct {
	authService AuthService
}

type AuthService interface {
	Refresh(ctx context.Context, input auth_service.RefreshInput) (auth_service.RefreshResult, error)
	Register(
		ctx context.Context,
		input auth_service.RegisterInput,
	) (auth_service.RegisterResult, error)
	Login(ctx context.Context, input auth_service.LoginInput) (auth_service.LoginResult, error)
	Logout(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) error
}

func NewHTTPAuthTransport(
	authService AuthService,
) *HTTPAuthTransport {
	return &HTTPAuthTransport{
		authService: authService,
	}
}
