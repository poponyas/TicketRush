package auth_service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *AuthService) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return LoginResult{}, core_errors.ErrInvalidArgument
	}

	password := input.Password
	passwordLength := utf8.RuneCountInString(input.Password)
	if passwordLength > 128 {
		return LoginResult{}, core_errors.ErrInvalidArgument
	}

	credentials, err := s.authRepo.GetCredentialsByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, core_errors.ErrNotFound) {
			if _, verifyErr := s.passwordHasher.Verify(password, s.dummyPasswordHash); verifyErr != nil {
				return LoginResult{}, fmt.Errorf("verify dummy password: %w", verifyErr)
			}
			return LoginResult{}, core_errors.ErrUnauthenticated
		}
		return LoginResult{}, fmt.Errorf("credential error: %w", err)
	}

	verified, err := s.passwordHasher.Verify(password, credentials.PasswordHash)
	if err != nil {
		return LoginResult{}, fmt.Errorf("verify password: %w", err)
	}
	if !verified {
		return LoginResult{}, core_errors.ErrUnauthenticated
	}

	sessionID := uuid.New()

	refreshToken := make([]byte, 32)
	if _, err := rand.Read(refreshToken); err != nil {
		return LoginResult{}, fmt.Errorf("generate refresh: %w", err)
	}

	refreshTokenEncoded := base64.RawURLEncoding.EncodeToString(refreshToken)

	refreshTokenHash := sha256.Sum256([]byte(refreshTokenEncoded))
	refreshTokenHashString := hex.EncodeToString(refreshTokenHash[:])

	accessToken, err := s.tokenManager.IssueAccess(
		credentials.ID,
		sessionID,
	)
	if err != nil {
		return LoginResult{}, fmt.Errorf("issue access token: %w", err)
	}
	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)

	if err := s.authRepo.CreateSession(
		ctx,
		sessionID,
		credentials.ID,
		refreshTokenHashString,
		expiresAt,
	); err != nil {
		return LoginResult{}, fmt.Errorf("create session: %w", err)
	}

	return LoginResult{
		ID:           credentials.ID,
		Email:        credentials.Email,
		AccessToken:  accessToken,
		RefreshToken: refreshTokenEncoded,
	}, nil
}
