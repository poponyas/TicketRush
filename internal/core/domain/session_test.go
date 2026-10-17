package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	"github.com/stretchr/testify/require"
)

func TestSessionValidation(t *testing.T) {
	start := time.Date(2027, 2, 15, 18, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	base := domain.NewSession(uuid.New(), uuid.New(), start, &end, start.Add(-48*time.Hour), start.Add(-time.Hour))
	base.CreatedAt = start.Add(-72 * time.Hour)
	require.NoError(t, base.Validate())
	for _, tc := range []struct {
		name   string
		change func(*domain.Session)
	}{
		{"missing ID", func(s *domain.Session) { s.ID = uuid.Nil }},
		{"missing event", func(s *domain.Session) { s.EventID = uuid.Nil }},
		{"missing venue", func(s *domain.Session) { s.VenueID = uuid.Nil }},
		{"missing start", func(s *domain.Session) { s.StartsAt = time.Time{} }},
		{"missing sales start", func(s *domain.Session) { s.SalesStartAt = time.Time{} }},
		{"missing sales end", func(s *domain.Session) { s.SalesEndAt = time.Time{} }},
		{"missing creation time", func(s *domain.Session) { s.CreatedAt = time.Time{} }},
		{"unknown status", func(s *domain.Session) { s.Status = "unknown" }},
		{"end equals start", func(s *domain.Session) { x := s.StartsAt; s.EndsAt = &x }},
		{"end before start", func(s *domain.Session) { x := s.StartsAt.Add(-time.Hour); s.EndsAt = &x }},
		{"sales equal", func(s *domain.Session) { s.SalesStartAt = s.SalesEndAt }},
		{"sales reversed", func(s *domain.Session) { s.SalesStartAt = s.SalesEndAt.Add(time.Hour) }},
		{"sales end after session start", func(s *domain.Session) { s.SalesEndAt = s.StartsAt.Add(time.Hour) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := base
			tc.change(&s)
			require.ErrorIs(t, s.Validate(), coreerrors.ErrInvalidArgument)
		})
	}
	base.EndsAt = nil
	base.SalesEndAt = base.StartsAt
	require.NoError(t, base.Validate())
}
