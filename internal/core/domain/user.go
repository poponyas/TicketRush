package domain

import (
	"github.com/google/uuid"
	"time"
)

// Issuer and Subject identify an account at the OIDC provider.
// Organizer rights belong to OrganizerMembership, not a client-supplied cookie.
type User struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	Issuer      string
	Subject     string
	CreatedAt   time.Time
}
