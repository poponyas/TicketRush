package venue_http_tranpsort

import (
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
)

type CreateVenueRequest struct {
	Name    string `json:"name"`
	City    string `json:"city"`
	Address string `json:"address"`
}
type VenueResponse struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	City    string    `json:"city"`
	Address string    `json:"address"`
}

type VenuesListResponse struct {
	Items  []VenueResponse `json:"items"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

func venueResponse(venue domain.Venue) VenueResponse {
	return VenueResponse{ID: venue.ID, Name: venue.Name, City: venue.City, Address: venue.Address}
}

func venuesResponse(venues []domain.Venue) []VenueResponse {
	venResponse := make([]VenueResponse, 0, len(venues))
	for _, event := range venues {
		ev := venueResponse(event)
		venResponse = append(venResponse, ev)
	}

	return venResponse
}
