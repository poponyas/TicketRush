package auth_repository

import "github.com/google/uuid"

type Credentials struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
}
