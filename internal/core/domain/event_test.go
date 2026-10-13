package domain_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	"github.com/stretchr/testify/require"
)

func TestEventValidation(t *testing.T) {
	for _, tc := range []struct {
		name, title, description, category string
		valid                              bool
	}{
		{"valid", "Go Live", "", "", true},
		{"unicode boundary", strings.Repeat("я", 200), strings.Repeat("д", 10000), strings.Repeat("к", 100), true},
		{"empty title", " ", "", "", false},
		{"long title", strings.Repeat("я", 201), "", "", false},
		{"long description", "Title", strings.Repeat("д", 10001), "", false},
		{"long category", "Title", "", strings.Repeat("к", 101), false},
		{"NUL title", "Go\x00Live", "", "", false},
		{"NUL description", "Title", "bad\x00text", "", false},
		{"NUL category", "Title", "", "bad\x00category", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := domain.NewEvent(uuid.New(), tc.title, tc.description, tc.category)
			if tc.valid {
				require.NoError(t, e.Validate())
			} else {
				require.ErrorIs(t, e.Validate(), coreerrors.ErrInvalidArgument)
			}
		})
	}
}
