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
	result, err := service.NewEventsService(repo, nil).GetPublicEvent(context.Background(), event.ID)
	require.NoError(t, err)
	require.Equal(t, event, result)
	require.Equal(t, 1, calls)
}

func TestGetPublicEventServiceRejectsNilID(t *testing.T) {
	// Empty stubs panic if validation lets the request reach either dependency.
	_, err := service.NewEventsService(&repositoryStub{}, &eventCacheStub{}).GetPublicEvent(context.Background(), uuid.Nil)
	require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
}

func TestGetPublicEventServicePreservesErrors(t *testing.T) {
	for _, expected := range []error{coreerrors.ErrNotFound, errors.New("database unavailable")} {
		repo := &repositoryStub{publicGet: func(context.Context, uuid.UUID) (domain.Event, error) {
			return domain.Event{}, fmt.Errorf("repository: %w", expected)
		}}
		_, err := service.NewEventsService(repo, nil).GetPublicEvent(context.Background(), uuid.New())
		require.ErrorIs(t, err, expected)
	}
}

type eventCacheStub struct {
	get func(context.Context, uuid.UUID) (domain.Event, bool, error)
	set func(context.Context, domain.Event) error
}

func (c *eventCacheStub) Get(ctx context.Context, id uuid.UUID) (domain.Event, bool, error) {
	if c.get == nil {
		panic("unexpected cache Get")
	}
	return c.get(ctx, id)
}

func (c *eventCacheStub) Set(ctx context.Context, event domain.Event) error {
	if c.set == nil {
		panic("unexpected cache Set")
	}
	return c.set(ctx, event)
}

func TestGetPublicEventServiceWithCache(t *testing.T) {
	databaseEvent := domain.NewEvent(uuid.New(), "Go Live", "Description", "conference")
	databaseEvent.Status = domain.EventPublished
	cachedEvent := databaseEvent
	cachedEvent.Title = "Cached Go Live"
	cacheError := errors.New("cache unavailable")
	databaseError := errors.New("database unavailable")

	for _, tc := range []struct {
		name          string
		found         bool
		getErr        error
		setErr        error
		repositoryErr error
		want          domain.Event
		wantErr       error
		wantCalls     []string
	}{
		{
			name:      "cache hit skips repository and cache write",
			found:     true,
			want:      cachedEvent,
			wantCalls: []string{"cache.get"},
		},
		{
			name:      "cache miss loads and caches database event",
			want:      databaseEvent,
			wantCalls: []string{"cache.get", "repository.get", "cache.set"},
		},
		{
			name:      "cache read error falls back to repository",
			getErr:    cacheError,
			want:      databaseEvent,
			wantCalls: []string{"cache.get", "repository.get", "cache.set"},
		},
		{
			name:      "cache write error preserves successful database result",
			setErr:    cacheError,
			want:      databaseEvent,
			wantCalls: []string{"cache.get", "repository.get", "cache.set"},
		},
		{
			name:      "cache outage preserves successful database result",
			getErr:    cacheError,
			setErr:    cacheError,
			want:      databaseEvent,
			wantCalls: []string{"cache.get", "repository.get", "cache.set"},
		},
		{
			name:          "missing database event is not cached",
			repositoryErr: coreerrors.ErrNotFound,
			wantErr:       coreerrors.ErrNotFound,
			wantCalls:     []string{"cache.get", "repository.get"},
		},
		{
			name:          "database error is returned even after cache error",
			getErr:        cacheError,
			repositoryErr: databaseError,
			wantErr:       databaseError,
			wantCalls:     []string{"cache.get", "repository.get"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			repo := &repositoryStub{publicGet: func(_ context.Context, id uuid.UUID) (domain.Event, error) {
				calls = append(calls, "repository.get")
				require.Equal(t, databaseEvent.ID, id)
				if tc.repositoryErr != nil {
					return domain.Event{}, fmt.Errorf("repository: %w", tc.repositoryErr)
				}
				return databaseEvent, nil
			}}
			cache := &eventCacheStub{
				get: func(_ context.Context, id uuid.UUID) (domain.Event, bool, error) {
					calls = append(calls, "cache.get")
					require.Equal(t, databaseEvent.ID, id)
					if tc.getErr != nil {
						return domain.Event{}, false, tc.getErr
					}
					if tc.found {
						return cachedEvent, true, nil
					}
					return domain.Event{}, false, nil
				},
				set: func(_ context.Context, event domain.Event) error {
					calls = append(calls, "cache.set")
					require.Equal(t, databaseEvent, event)
					return tc.setErr
				},
			}

			result, err := service.NewEventsService(repo, cache).GetPublicEvent(context.Background(), databaseEvent.ID)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.want, result)
			require.Equal(t, tc.wantCalls, calls)
		})
	}
}
