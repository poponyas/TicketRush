package posters_service

import (
	"bytes"
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"time"
)

const MaxSize = 5 << 20

type Events interface {
	GetEvent(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Event, error)
	GetPublicEvent(context.Context, uuid.UUID) (domain.Event, error)
	SetPoster(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) error
}
type Objects interface {
	Put(context.Context, string, []byte, string) error
	Get(context.Context, string) ([]byte, string, error)
	Delete(context.Context, string) error
}
type Service struct {
	events  Events
	objects Objects
}

func New(events Events, objects Objects) *Service { return &Service{events: events, objects: objects} }
func (s *Service) Upload(ctx context.Context, event, org, user uuid.UUID, data []byte) (string, error) {
	if event == uuid.Nil || org == uuid.Nil || user == uuid.Nil {
		return "", coreerrors.ErrInvalidArgument
	}
	if len(data) > MaxSize {
		return "", coreerrors.ErrPayloadTooLarge
	}
	if _, err := s.events.GetEvent(ctx, event, org, user); err != nil {
		return "", err
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 10000 || cfg.Height > 10000 || int64(cfg.Width)*int64(cfg.Height) > 20_000_000 {
		return "", coreerrors.ErrInvalidArgument
	}
	if format != "png" && format != "jpeg" {
		return "", coreerrors.ErrUnsupportedMediaType
	}
	// Fully decode after the pixel bound check to reject truncated/corrupt images.
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return "", coreerrors.ErrInvalidArgument
	}
	key := fmt.Sprintf("posters/%s/%s.%s", event, uuid.New(), format)
	if err := s.objects.Put(ctx, key, data, "image/"+format); err != nil {
		return "", err
	}
	if err := s.events.SetPoster(ctx, event, org, user, key); err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.objects.Delete(cleanup, key)
		return "", err
	}
	return "/api/v1/events/" + event.String() + "/poster", nil
}
func (s *Service) Get(ctx context.Context, event uuid.UUID) ([]byte, string, error) {
	if event == uuid.Nil {
		return nil, "", coreerrors.ErrInvalidArgument
	}
	e, err := s.events.GetPublicEvent(ctx, event)
	if err != nil {
		return nil, "", err
	}
	if e.PosterKey == "" {
		return nil, "", coreerrors.ErrNotFound
	}
	return s.objects.Get(ctx, e.PosterKey)
}
