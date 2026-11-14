package account_service

import (
	"context"
	"github.com/google/uuid"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	"github.com/poponyas/TicketRush/internal/feature/account"
)

type Repository interface {
	Profile(context.Context, uuid.UUID) (account.Profile, error)
	Bookings(context.Context, uuid.UUID, int, int) ([]account.Booking, error)
}
type Service struct{ repo Repository }

func New(repo Repository) *Service { return &Service{repo: repo} }
func (s *Service) Profile(ctx context.Context, userID uuid.UUID) (account.Profile, error) {
	if userID == uuid.Nil {
		return account.Profile{}, coreerrors.ErrUnauthenticated
	}
	return s.repo.Profile(ctx, userID)
}
func (s *Service) Bookings(ctx context.Context, userID uuid.UUID, limit, offset int) (account.BookingPage, error) {
	if userID == uuid.Nil {
		return account.BookingPage{}, coreerrors.ErrUnauthenticated
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return account.BookingPage{}, coreerrors.ErrInvalidArgument
	}
	items, err := s.repo.Bookings(ctx, userID, limit, offset)
	return account.BookingPage{Items: items, Limit: limit, Offset: offset}, err
}
