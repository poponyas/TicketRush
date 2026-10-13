package venue_repository_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	"github.com/poponyas/TicketRush/internal/feature/venue/testutil"
	"github.com/stretchr/testify/require"
)

func TestVenueReadsPostgres(t *testing.T) {
	pool, repo := testutil.Database(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	venues, err := repo.GetVenues(ctx, 20, 0)
	require.NoError(t, err)
	require.Empty(t, venues)
	_, err = repo.GetVenue(ctx, uuid.New())
	require.ErrorIs(t, err, coreerrors.ErrNotFound)
	var expected []domain.Venue
	// Equal names force the ID tie-breaker to participate in ordering.
	for _, name := range []string{"B Hall", "A Hall", "A Hall"} {
		venue := domain.NewVenue(name, "Moscow", "Address")
		saved, err := repo.CreateVenue(ctx, venue)
		require.NoError(t, err)
		require.Equal(t, venue, saved)
		read, err := repo.GetVenue(ctx, saved.ID)
		require.NoError(t, err)
		require.Equal(t, saved, read)
		expected = append(expected, saved)
	}
	sort.Slice(expected, func(i, j int) bool {
		if expected[i].Name == expected[j].Name {
			return expected[i].ID.String() < expected[j].ID.String()
		}
		return expected[i].Name < expected[j].Name
	})
	venues, err = repo.GetVenues(ctx, 2, 0)
	require.NoError(t, err)
	require.Equal(t, expected[:2], venues)
	venues, err = repo.GetVenues(ctx, 2, 2)
	require.NoError(t, err)
	require.Equal(t, expected[2:], venues)
	venues, err = repo.GetVenues(ctx, 2, 3)
	require.NoError(t, err)
	require.Empty(t, venues)
	// SQL failures must stay failures instead of returning an empty successful page.
	_, err = pool.Exec(ctx, `DROP TABLE venues`)
	require.NoError(t, err)
	_, err = repo.GetVenues(ctx, 20, 0)
	require.Error(t, err)
	_, err = repo.GetVenue(ctx, expected[0].ID)
	require.Error(t, err)
	require.NotErrorIs(t, err, coreerrors.ErrNotFound)
}
