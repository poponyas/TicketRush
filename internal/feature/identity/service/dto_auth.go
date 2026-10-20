package auth_service

import "github.com/google/uuid"

type RegisterInput struct {
	Email    string
	Password string
}

type RegisterResult struct {
	ID    uuid.UUID
	Email string
}

type LoginInput struct {
	Email    string
	Password string
}

type LoginResult struct {
	ID           uuid.UUID
	Email        string
	AccessToken  string
	RefreshToken string
}

type RefreshInput struct{ RefreshToken string }
type RefreshResult struct {
	AccessToken  string
	RefreshToken string
}
