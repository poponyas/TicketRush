// Package ticketqr signs QR payloads with a key derived for this purpose only.
package ticketqr

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/uuid"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

type Signer struct{ key []byte }

func New(secret []byte) (*Signer, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("QR signing secret must be at least 32 bytes")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte("ticketrush/ticket-qr/v1"))
	return &Signer{key: mac.Sum(nil)}, nil
}
func (s *Signer) Issue(id uuid.UUID) (string, error) {
	if id == uuid.Nil {
		return "", coreerrors.ErrInvalidArgument
	}
	message := "v1." + id.String()
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(message))
	return message + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func (s *Signer) Verify(token string) (uuid.UUID, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != "v1" {
		return uuid.Nil, coreerrors.ErrInvalidArgument
	}
	id, err := uuid.Parse(parts[1])
	if err != nil || id == uuid.Nil || id.String() != parts[1] {
		return uuid.Nil, coreerrors.ErrInvalidArgument
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return uuid.Nil, coreerrors.ErrInvalidArgument
	}
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return uuid.Nil, coreerrors.ErrInvalidArgument
	}
	return id, nil
}
