package auth_repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (r *AuthRepository) Register(
	ctx context.Context,
	userID uuid.UUID,
	passHash string,
	email string,
) (uuid.UUID, string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout)
	defer cancel()

	const query = `
        INSERT INTO users (id, email, password_hash)
        VALUES ($1, $2, $3)
        RETURNING id, email
    `

	var savedID uuid.UUID
	var savedEmail string

	err := r.pool.Pool.QueryRow(
		ctx, query, userID, email, passHash,
	).Scan(&savedID, &savedEmail)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "users_email_key" {
			return uuid.Nil, "", fmt.Errorf(
				"email already registered: %w",
				core_errors.ErrConflict,
			)
		}

		return uuid.Nil, "", fmt.Errorf("insert user: %w", err)
	}

	return savedID, savedEmail, nil
}
