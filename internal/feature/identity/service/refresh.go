package auth_service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *AuthService) Refresh(ctx context.Context, input RefreshInput) (RefreshResult, error) {
	// Accept only the canonical, 32-byte base64url tokens issued by Login.
	if len(input.RefreshToken) != base64.RawURLEncoding.EncodedLen(32) {
		return RefreshResult{}, coreerrors.ErrUnauthenticated
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(input.RefreshToken)
	if err != nil || len(decoded) != 32 || base64.RawURLEncoding.EncodeToString(decoded) != input.RefreshToken {
		return RefreshResult{}, coreerrors.ErrUnauthenticated
	}
	digest := sha256.Sum256([]byte(input.RefreshToken))
	oldHash := hex.EncodeToString(digest[:])
	session, err := s.authRepo.GetActiveSessionByRefreshHash(ctx, oldHash)
	if err != nil {
		if errors.Is(err, coreerrors.ErrNotFound) {
			return RefreshResult{}, coreerrors.ErrUnauthenticated
		}
		return RefreshResult{}, fmt.Errorf("get refresh session: %w", err)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return RefreshResult{}, fmt.Errorf("generate refresh token: %w", err)
	}
	refreshToken := base64.RawURLEncoding.EncodeToString(raw)
	newDigest := sha256.Sum256([]byte(refreshToken))
	newHash := hex.EncodeToString(newDigest[:])

	// Sign before rotating: a signing failure must leave the old refresh usable.
	accessToken, err := s.tokenManager.IssueAccess(session.UserID, session.ID)
	if err != nil {
		return RefreshResult{}, fmt.Errorf("issue refreshed access token: %w", err)
	}
	if err := s.authRepo.RotateRefreshToken(ctx, session.ID, session.UserID, oldHash, newHash); err != nil {
		// This also covers a concurrent refresh, revocation or expiration.
		if errors.Is(err, coreerrors.ErrNotFound) {
			return RefreshResult{}, coreerrors.ErrUnauthenticated
		}
		return RefreshResult{}, fmt.Errorf("rotate refresh token: %w", err)
	}
	return RefreshResult{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}
