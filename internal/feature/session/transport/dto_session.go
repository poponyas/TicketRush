package session_transport

import (
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
)

type CreateSessionRequest struct {
	VenueID      uuid.UUID  `json:"venue_id"`
	StartsAt     time.Time  `json:"starts_at"`
	EndsAt       *time.Time `json:"ends_at"`
	SalesStartAt time.Time  `json:"sales_start_at"`
	SalesEndAt   time.Time  `json:"sales_end_at"`
}

type UpdateSessionRequest struct {
	StartsAt  *time.Time `json:"starts_at"`
	EndsAt    *time.Time `json:"ends_at"`
	EndsAtSet bool       `json:"-"`
}

type SessionResponse struct {
	ID           uuid.UUID            `json:"id"`
	EventID      uuid.UUID            `json:"event_id"`
	VenueID      uuid.UUID            `json:"venue_id"`
	StartsAt     time.Time            `json:"starts_at"`
	EndsAt       *time.Time           `json:"ends_at"`
	SalesStartAt time.Time            `json:"sales_start_at"`
	SalesEndAt   time.Time            `json:"sales_end_at"`
	Status       domain.SessionStatus `json:"status"`
	CreatedAt    time.Time            `json:"created_at"`
}

type SessionsListResponse struct {
	Items  []SessionResponse `json:"items"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

func sessionsToDTO(sessions []domain.Session) []SessionResponse {
	items := make([]SessionResponse, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, domainSessionToDTO(session))
	}
	return items
}

func domainSessionToDTO(session domain.Session) SessionResponse {
	return SessionResponse{
		ID:           session.ID,
		EventID:      session.EventID,
		VenueID:      session.VenueID,
		StartsAt:     session.StartsAt,
		EndsAt:       session.EndsAt,
		SalesStartAt: session.SalesStartAt,
		SalesEndAt:   session.SalesEndAt,
		Status:       session.Status,
		CreatedAt:    session.CreatedAt,
	}
}
