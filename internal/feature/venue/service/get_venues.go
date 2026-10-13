package venue_service

import (
	"context"
	"fmt"

	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *VenueService) GetVenues(ctx context.Context, limit, offset *int) (VenuesPage, error) {
	page := VenuesPage{Limit: 20}
	if limit != nil {
		page.Limit = *limit
	}
	if offset != nil {
		page.Offset = *offset
	}
	if page.Limit < 1 || page.Limit > 100 {
		return VenuesPage{}, fmt.Errorf("%w: limit must be between 1 and 100", coreerrors.ErrInvalidArgument)
	}
	if page.Offset < 0 {
		return VenuesPage{}, fmt.Errorf("%w: offset must be nonnegative", coreerrors.ErrInvalidArgument)
	}
	venues, err := s.venueRepo.GetVenues(ctx, page.Limit, page.Offset)
	if err != nil {
		return VenuesPage{}, fmt.Errorf("list venues: %w", err)
	}
	page.Venues = venues
	return page, nil
}
