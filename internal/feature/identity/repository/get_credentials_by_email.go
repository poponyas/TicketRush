package auth_repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *AuthRepository) GetCredentialsByEmail(ctx context.Context, email string) (Credentials, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()

	query := `
	SELECT id, email, password_hash
	FROM users
	WHERE email = $1;
	`

	var id uuid.UUID
	var emailByRepo string
	var passwordHash string
	if err := r.pool.Pool.QueryRow(ctx, query, email).Scan(
		&id,
		&emailByRepo,
		&passwordHash,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Credentials{}, fmt.Errorf("no rows: %w", core_errors.ErrNotFound)
		}
		return Credentials{}, fmt.Errorf("scan error: %w", err)
	}

	return Credentials{
		ID:           id,
		Email:        emailByRepo,
		PasswordHash: passwordHash,
	}, nil
}
