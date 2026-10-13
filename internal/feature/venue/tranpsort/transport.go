package venue_http_tranpsort

import (
	"context"

	"github.com/google/uuid"

	"github.com/poponyas/TicketRush/internal/core/domain"
	service "github.com/poponyas/TicketRush/internal/feature/venue/service"
)

type HTTPVenueTransport struct {
	venueService VenueService
}

type VenueService interface {
	CreateVenue(ctx context.Context, name, city, address string) (domain.Venue, error)
	GetVenue(ctx context.Context, id uuid.UUID) (domain.Venue, error)
	GetVenues(ctx context.Context, limit, offset *int) (service.VenuesPage, error)
}

func NewHTTPVenueTransport(
	venueService VenueService,
) *HTTPVenueTransport {
	return &HTTPVenueTransport{
		venueService: venueService,
	}
}
