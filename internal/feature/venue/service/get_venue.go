package venue_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

func (s *VenueService) GetVenue(ctx context.Context, id uuid.UUID) (domain.Venue, error) {
	if id == uuid.Nil {
		return domain.Venue{}, fmt.Errorf("%w: venue ID is required", coreerrors.ErrInvalidArgument)
	}
	venue, err := s.venueRepo.GetVenue(ctx, id)
	if err != nil {
		return domain.Venue{}, fmt.Errorf("get venue: %w", err)
	}
	return venue, nil
}
