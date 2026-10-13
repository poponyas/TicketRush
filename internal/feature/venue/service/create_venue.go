package venue_service

import (
	"context"
	"fmt"

	"github.com/poponyas/TicketRush/internal/core/domain"
)

func (s *VenueService) CreateVenue(ctx context.Context, name, city, address string) (domain.Venue, error) {
	venue := domain.NewVenue(
		name,
		city,
		address,
	)

	if err := venue.Validate(); err != nil {
		return domain.Venue{}, fmt.Errorf("validate err: %w", err)
	}

	response, err := s.venueRepo.CreateVenue(ctx, venue)
	if err != nil {
		return domain.Venue{}, fmt.Errorf("create venue err: %w", err)
	}

	return response, nil
}
