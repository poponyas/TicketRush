package venue_service

import (
	"context"

	"github.com/google/uuid"

	"github.com/poponyas/TicketRush/internal/core/domain"
)

type VenueService struct {
	venueRepo VenueRepository
}

type VenueRepository interface {
	CreateVenue(ctx context.Context, venue domain.Venue) (domain.Venue, error)
	GetVenue(ctx context.Context, id uuid.UUID) (domain.Venue, error)
	GetVenues(ctx context.Context, limit, offset int) ([]domain.Venue, error)
}

type VenuesPage struct {
	Venues []domain.Venue
	Limit  int
	Offset int
}

func NewVenueService(
	venueRepo VenueRepository,
) *VenueService {
	return &VenueService{
		venueRepo: venueRepo,
	}
}
