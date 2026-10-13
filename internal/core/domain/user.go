package domain

import (
	"github.com/google/uuid"
	"time"
)

// Issuer and Subject identify an account at the OIDC provider.
// Organizer rights belong to OrganizerMembership, not a client-supplied cookie.
// Пользователь сервиса
type User struct {
	ID          uuid.UUID `db:"id"`
	Email       string    `db:"email"`
	DisplayName string    `db:"display_name"`
	Issuer      string    `db:"issuer"`
	Subject     string    `db:"subject"`
	CreatedAt   time.Time `db:"created_at"`
}
