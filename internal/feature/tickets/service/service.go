package tickets_service

import (
	"context"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	"github.com/skip2/go-qrcode"
)

type Repository interface {
	Get(context.Context, uuid.UUID, uuid.UUID) (domain.Ticket, error)
	List(context.Context, uuid.UUID, int, int) ([]domain.Ticket, error)
	CheckIn(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Ticket, error)
}
type Signer interface {
	Issue(uuid.UUID) (string, error)
	Verify(string) (uuid.UUID, error)
}
type Service struct {
	repo   Repository
	signer Signer
}

func New(repo Repository, signer Signer) *Service { return &Service{repo: repo, signer: signer} }

type Detail struct {
	Ticket    domain.Ticket
	QRPayload string
}

func (s *Service) Get(ctx context.Context, id, user uuid.UUID) (Detail, error) {
	if id == uuid.Nil || user == uuid.Nil {
		return Detail{}, coreerrors.ErrInvalidArgument
	}
	ticket, err := s.repo.Get(ctx, id, user)
	if err != nil {
		return Detail{}, err
	}
	payload, err := s.signer.Issue(ticket.ID)
	if err != nil {
		return Detail{}, err
	}
	return Detail{Ticket: ticket, QRPayload: payload}, nil
}
func (s *Service) List(ctx context.Context, user uuid.UUID, limit, offset int) ([]domain.Ticket, error) {
	if user == uuid.Nil || limit < 1 || limit > 100 || offset < 0 {
		return nil, coreerrors.ErrInvalidArgument
	}
	return s.repo.List(ctx, user, limit, offset)
}
func (s *Service) QR(ctx context.Context, id, user uuid.UUID) ([]byte, error) {
	detail, err := s.Get(ctx, id, user)
	if err != nil {
		return nil, err
	}
	if detail.Ticket.Status != domain.TicketActive {
		return nil, coreerrors.ErrConflict
	}
	return qrcode.Encode(detail.QRPayload, qrcode.Medium, 320)
}
func (s *Service) CheckIn(ctx context.Context, token string, session, user uuid.UUID) (domain.Ticket, error) {
	if session == uuid.Nil || user == uuid.Nil {
		return domain.Ticket{}, coreerrors.ErrInvalidArgument
	}
	id, err := s.signer.Verify(token)
	if err != nil {
		return domain.Ticket{}, err
	}
	return s.repo.CheckIn(ctx, id, session, user)
}
