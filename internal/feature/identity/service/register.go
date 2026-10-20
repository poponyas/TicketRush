package auth_service

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *AuthService) Register(
	ctx context.Context,
	input RegisterInput,
) (RegisterResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return RegisterResult{}, core_errors.ErrInvalidArgument
	}

	password := input.Password
	passwordLength := utf8.RuneCountInString(input.Password)
	if passwordLength < 15 || passwordLength > 128 {
		return RegisterResult{}, core_errors.ErrInvalidArgument
	}

	passHash, err := s.passwordHasher.Hash(password)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("hash error: %w", err)
	}

	userID := uuid.New()

	IDByRepo, emailByRepo, err := s.authRepo.Register(ctx, userID, passHash, email)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("register error: %w", err)
	}

	return RegisterResult{ID: IDByRepo, Email: emailByRepo}, nil
}
