package posters_service_test

import (
	"bytes"
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/posters/service"
	"github.com/stretchr/testify/require"
	"image"
	"image/png"
	"testing"
)

type events struct {
	access, save error
	key          string
}

func (e *events) GetEvent(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Event, error) {
	return domain.Event{}, e.access
}
func (e *events) GetPublicEvent(context.Context, uuid.UUID) (domain.Event, error) {
	return domain.Event{PosterKey: e.key}, e.access
}
func (e *events) SetPoster(_ context.Context, _, _, _ uuid.UUID, key string) error {
	if e.save == nil {
		e.key = key
	}
	return e.save
}

type objects struct {
	put, deleted int
	err          error
}

func (o *objects) Put(context.Context, string, []byte, string) error { o.put++; return o.err }
func (o *objects) Get(context.Context, string) ([]byte, string, error) {
	return []byte("image"), "image/png", o.err
}
func (o *objects) Delete(context.Context, string) error { o.deleted++; return nil }
func TestPosterUpload(t *testing.T) {
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	for _, mode := range []string{"success", "no access", "corrupt", "too large", "storage down", "save failed"} {
		t.Run(mode, func(t *testing.T) {
			e, o := &events{}, &objects{}
			data := buffer.Bytes()
			expected := error(nil)
			switch mode {
			case "no access":
				e.access = coreerrors.ErrNotFound
				expected = e.access
			case "corrupt":
				data = []byte("bad")
				expected = coreerrors.ErrInvalidArgument
			case "too large":
				data = make([]byte, service.MaxSize+1)
				expected = coreerrors.ErrPayloadTooLarge
			case "storage down":
				o.err = coreerrors.ErrUnavailable
				expected = o.err
			case "save failed":
				e.save = errors.New("database failed")
				expected = e.save
			}
			url, err := service.New(e, o).Upload(context.Background(), uuid.New(), uuid.New(), uuid.New(), data)
			if expected != nil {
				require.ErrorIs(t, err, expected)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, url)
				require.NotEmpty(t, e.key)
			}
			if mode == "save failed" {
				require.Equal(t, 1, o.deleted)
			}
			if mode == "no access" || mode == "corrupt" || mode == "too large" {
				require.Zero(t, o.put)
			}
		})
	}
}
