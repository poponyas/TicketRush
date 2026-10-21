package events_service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/events/service"
	"github.com/stretchr/testify/require"
)

func TestGetPublicEventService(t *testing.T) {
	event := domain.NewEvent(uuid.New(), "Go Live", "Description", "conference")
	event.Status = domain.EventPublished
	calls := 0
	repo := &repositoryStub{publicGet: func(ctx context.Context, id uuid.UUID) (domain.Event, error) {
		calls++
		require.Equal(t, event.ID, id)
		return event, nil
	}}
	result, err := service.NewEventsService(repo).GetPublicEvent(context.Background(), event.ID)
	require.NoError(t, err)
	require.Equal(t, event, result)
	require.Equal(t, 1, calls)
}

func TestGetPublicEventServiceRejectsNilID(t *testing.T) {
	_, err := service.NewEventsService(&repositoryStub{}).GetPublicEvent(context.Background(), uuid.Nil)
	require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
}

func TestGetPublicEventServicePreservesErrors(t *testing.T) {
	for _, expected := range []error{coreerrors.ErrNotFound, errors.New("database unavailable")} {
		repo := &repositoryStub{publicGet: func(context.Context, uuid.UUID) (domain.Event, error) {
			return domain.Event{}, fmt.Errorf("repository: %w", expected)
		}}
		_, err := service.NewEventsService(repo).GetPublicEvent(context.Background(), uuid.New())
		require.ErrorIs(t, err, expected)
	}
}
