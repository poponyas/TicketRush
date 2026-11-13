package ticketqr_test

import (
	"bytes"
	"github.com/google/uuid"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	"github.com/poponyas/TicketRush/internal/core/security/ticketqr"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestTicketQR(t *testing.T) {
	s, err := ticketqr.New(bytes.Repeat([]byte{1}, 32))
	require.NoError(t, err)
	id := uuid.New()
	token, err := s.Issue(id)
	require.NoError(t, err)
	got, err := s.Verify(token)
	require.NoError(t, err)
	require.Equal(t, id, got)
	same, err := s.Issue(id)
	require.NoError(t, err)
	require.Equal(t, token, same)
	other, _ := ticketqr.New(bytes.Repeat([]byte{2}, 32))
	_, err = other.Verify(token)
	require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	parts := strings.Split(token, ".")
	parts[1] = uuid.NewString()
	for _, bad := range []string{"", strings.Join(parts, "."), "v2" + token[2:], token + ".extra", "v1." + uuid.Nil.String() + ".x"} {
		_, err = s.Verify(bad)
		require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	}
	_, err = s.Issue(uuid.Nil)
	require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
	_, err = ticketqr.New(nil)
	require.Error(t, err)
}
