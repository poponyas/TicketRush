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

func TestGetPublicEventsPagination(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		limit, offset         *int
		wantLimit, wantOffset int
	}{
		{"defaults", nil, nil, 20, 0},
		{"minimum", numberPointer(1), numberPointer(0), 1, 0},
		{"maximum and offset", numberPointer(100), numberPointer(37), 100, 37},
		{"offset only", nil, numberPointer(5), 20, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expected := []domain.Event{{ID: uuid.New(), Status: domain.EventPublished}}
			calls := 0
			repo := &repositoryStub{publicList: func(ctx context.Context, limit, offset int) ([]domain.Event, error) {
				calls++
				require.Equal(t, tc.wantLimit, limit)
				require.Equal(t, tc.wantOffset, offset)
				return expected, nil
			}}
			page, err := service.NewEventsService(repo).GetPublicEvents(context.Background(), tc.limit, tc.offset)
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			require.Equal(t, expected, page.Events)
			require.Equal(t, tc.wantLimit, page.Limit)
			require.Equal(t, tc.wantOffset, page.Offset)
		})
	}
}

func TestGetPublicEventsRejectsInvalidPagination(t *testing.T) {
	for _, tc := range []struct {
		name          string
		limit, offset *int
	}{
		{"zero limit", numberPointer(0), nil},
		{"negative limit", numberPointer(-1), nil},
		{"too large limit", numberPointer(101), nil},
		{"negative offset", nil, numberPointer(-1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// An unexpected repository call panics: invalid input must stop here.
			_, err := service.NewEventsService(&repositoryStub{}).GetPublicEvents(context.Background(), tc.limit, tc.offset)
			require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
		})
	}
}

func TestGetPublicEventsPreservesRepositoryError(t *testing.T) {
	expected := errors.New("database unavailable")
	repo := &repositoryStub{publicList: func(context.Context, int, int) ([]domain.Event, error) {
		return nil, fmt.Errorf("query: %w", expected)
	}}
	_, err := service.NewEventsService(repo).GetPublicEvents(context.Background(), nil, nil)
	require.ErrorIs(t, err, expected)
}
