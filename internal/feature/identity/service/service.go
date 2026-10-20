package auth_service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	auth_repository "github.com/poponyas/TicketRush/internal/feature/identity/repository"
)

type AuthService struct {
	authRepo          AuthRepository
	passwordHasher    PasswordHasher
	tokenManager      TokenManager
	dummyPasswordHash string
}

type AuthRepository interface {
	GetActiveSessionByRefreshHash(ctx context.Context, hash string) (domain.AuthSession, error)
	RotateRefreshToken(ctx context.Context, sessionID, userID uuid.UUID, oldHash, newHash string) error
	GetOrganizerRole(ctx context.Context, organizerID, userID uuid.UUID) (domain.Role, error)
	GetActiveSession(ctx context.Context, sessionID, userID uuid.UUID) (domain.AuthSession, error)
	Register(ctx context.Context, userID uuid.UUID, passHash string, email string) (uuid.UUID, string, error)
	GetCredentialsByEmail(ctx context.Context, email string) (auth_repository.Credentials, error)
	CreateSession(
		ctx context.Context,
		sessionID uuid.UUID,
		userID uuid.UUID,
		refreshHash string,
		expiresAt time.Time,
	) error
	RevokeSession(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) error
}

func NewAuthService(
	authRepo AuthRepository,
	passwordHasher PasswordHasher,
	tokenManager TokenManager,
) (*AuthService, error) {
	dummyHash, err := passwordHasher.Hash("TicketRush dummy password for unknown users")
	if err != nil {
		return nil, fmt.Errorf("create dummy password hash: %w", err)
	}
	return &AuthService{
		authRepo:          authRepo,
		passwordHasher:    passwordHasher,
		tokenManager:      tokenManager,
		dummyPasswordHash: dummyHash,
	}, nil
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password string, checkedString string) (bool, error)
}

type TokenManager interface {
	IssueAccess(
		userID uuid.UUID,
		sessionID uuid.UUID,
	) (string, error)
}
