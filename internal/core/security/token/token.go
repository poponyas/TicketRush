package auth_token

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/golang-jwt/jwt/v5"
)

type JWTConfig struct {
	Secret []byte
}

func LoadJWTConfig() (JWTConfig, error) {
	raw := os.Getenv("JWT_SECRET_HEX")

	secret, err := hex.DecodeString(raw)
	if err != nil {
		return JWTConfig{}, fmt.Errorf("decode JWT secret: %w", err)
	}

	if len(secret) < 32 {
		return JWTConfig{}, fmt.Errorf(
			"JWT secret must be at least 32 bytes",
		)
	}

	return JWTConfig{Secret: secret}, nil
}

type Manager struct {
	secret    []byte
	issuer    string
	audience  string
	accessTTL time.Duration
}

func NewManager(
	secret []byte,
	issuer string,
	audience string,
	accessTTL time.Duration,
) (*Manager, error) {

	if len(secret) < 32 {
		return nil, errors.New("JWT secret must be at least 32 bytes")
	}

	if issuer == "" || audience == "" {
		return nil, errors.New("issuer and audience are required")
	}

	if accessTTL <= 0 {
		return nil, errors.New("access TTL must be positive")
	}

	return &Manager{
		secret:    bytes.Clone(secret),
		issuer:    issuer,
		audience:  audience,
		accessTTL: accessTTL,
	}, nil
}

type AccessClaims struct {
	SessionID uuid.UUID `json:"session_id"`
	jwt.RegisteredClaims
}

func (m *Manager) IssueAccess(
	userID uuid.UUID,
	sessionID uuid.UUID,
) (string, error) {
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return "", fmt.Errorf("IDs must be not nil")
	}

	now := time.Now().UTC()

	claims := AccessClaims{
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	return token.SignedString(m.secret)
}

func (m *Manager) VerifyAccess(rawToken string) (uuid.UUID, uuid.UUID, error) {
	claims := AccessClaims{}
	token, err := jwt.ParseWithClaims(rawToken, &claims, func(token *jwt.Token) (any, error) {
		return m.secret, nil
	},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(m.audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse claims error: %w", err)
	}

	if !token.Valid {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid token")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("userID parse error: %w", err)
	}

	if userID == uuid.Nil || claims.SessionID == uuid.Nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("IDs must be not nil")
	}

	return userID, claims.SessionID, nil
}
