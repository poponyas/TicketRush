package domain

import (
	"github.com/google/uuid"
	"time"
)

// AuthSession describes a login session, not an event session.
// Refresh token hashes remain internal to the repository.
type AuthSession struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}
