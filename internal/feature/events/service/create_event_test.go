package events_service_test

import (
	"context"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/events/service"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

type recordingRepo struct {
	saved domain.Event
	actor uuid.UUID
	calls int
}

func (r *recordingRepo) GetEvent(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Event, error) {
	panic("unexpected GetEvent call in CreateEvent test")
}

func (r *recordingRepo) GetEvents(context.Context, int, int, uuid.UUID, uuid.UUID) ([]domain.Event, error) {
	panic("unexpected GetEvents call in CreateEvent test")
}

func (r *recordingRepo) UpdateEvent(context.Context, domain.Event, uuid.UUID) (domain.Event, error) {
	panic("unexpected UpdateEvent call in CreateEvent test")
}

func (r *recordingRepo) DeleteEvent(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	panic("unexpected DeleteEvent call in CreateEvent test")
}

func (r *recordingRepo) CreateEvent(ctx context.Context, e domain.Event, actor uuid.UUID) (domain.Event, error) {
	r.calls++
	r.saved = e
	r.actor = actor
	return e, nil
}
func TestCreateEventNormalizesDraft(t *testing.T) {
	repo := &recordingRepo{}
	s := service.NewEventsService(repo)
	input := service.CreateEventInput{UserID: uuid.New(), OrganizerID: uuid.New(), Title: " Go Live ", Description: " Desc ", Category: " conference "}
	event, err := s.CreateEvent(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "Go Live", event.Title)
	require.Equal(t, "Desc", event.Description)
	require.Equal(t, domain.EventDraft, event.Status)
	require.Equal(t, input.OrganizerID, event.OrganizerID)
	require.Equal(t, input.UserID, repo.actor)
	require.NotEqual(t, uuid.Nil, event.ID)
	require.True(t, event.CreatedAt.IsZero(), "creation time belongs to the database")
}
func TestInvalidEventDoesNotReachRepository(t *testing.T) {
	for _, name := range []string{" ", strings.Repeat("я", 201)} {
		t.Run(name, func(t *testing.T) {
			repo := &recordingRepo{}
			s := service.NewEventsService(repo)
			_, err := s.CreateEvent(context.Background(), service.CreateEventInput{UserID: uuid.New(), OrganizerID: uuid.New(), Title: name})
			require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
			require.Zero(t, repo.calls)
		})
	}
	repo := &recordingRepo{}
	s := service.NewEventsService(repo)
	_, err := s.CreateEvent(context.Background(), service.CreateEventInput{UserID: uuid.New(), OrganizerID: uuid.New(), Title: strings.Repeat("я", 200)})
	require.NoError(t, err)
}
