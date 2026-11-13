// Package checkout_service exposes an explicitly enabled mock payment flow.
package checkout_service

import (
	"context"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

type Repository interface {
	ConfirmBooking(context.Context, uuid.UUID, uuid.UUID) (domain.Booking, error)
}
type Service struct {
	repo    Repository
	enabled bool
}

func New(repo Repository, enabled bool) *Service { return &Service{repo: repo, enabled: enabled} }
func (s *Service) Confirm(ctx context.Context, bookingID, userID uuid.UUID) (domain.Booking, error) {
	if !s.enabled {
		return domain.Booking{}, coreerrors.ErrForbidden
	}
	if bookingID == uuid.Nil || userID == uuid.Nil {
		return domain.Booking{}, coreerrors.ErrInvalidArgument
	}
	return s.repo.ConfirmBooking(ctx, bookingID, userID)
}
