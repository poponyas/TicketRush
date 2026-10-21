package events_service_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	service "github.com/poponyas/TicketRush/internal/feature/events/service"
	"github.com/stretchr/testify/require"
)

// Unexpected database calls fail the test: invalid input must stop in the service.
type repositoryStub struct {
	get    func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Event, error)
	list   func(context.Context, int, int, uuid.UUID, uuid.UUID) ([]domain.Event, error)
	update func(context.Context, domain.Event, uuid.UUID) (domain.Event, error)
	delete func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
}

func (r *repositoryStub) CreateEvent(context.Context, domain.Event, uuid.UUID) (domain.Event, error) {
	panic("unexpected CreateEvent")
}
func (r *repositoryStub) GetEvent(ctx context.Context, id, org, user uuid.UUID) (domain.Event, error) {
	return r.get(ctx, id, org, user)
}
func (r *repositoryStub) GetEvents(ctx context.Context, limit, offset int, org, user uuid.UUID) ([]domain.Event, error) {
	return r.list(ctx, limit, offset, org, user)
}
func (r *repositoryStub) UpdateEvent(ctx context.Context, event domain.Event, user uuid.UUID) (domain.Event, error) {
	return r.update(ctx, event, user)
}
func (r *repositoryStub) DeleteEvent(ctx context.Context, id, org, user uuid.UUID) error {
	return r.delete(ctx, id, org, user)
}
func textPointer(s string) *string { return &s }
func numberPointer(n int) *int     { return &n }

func TestListPagination(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name                  string
		limit, offset         *int
		wantLimit, wantOffset int
	}{
		{"defaults", nil, nil, 20, 0}, {"first", numberPointer(1), numberPointer(0), 1, 0}, {"last allowed", numberPointer(100), numberPointer(10), 100, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			repo := &repositoryStub{list: func(ctx context.Context, limit, offset int, orgID, userID uuid.UUID) ([]domain.Event, error) {
				calls++
				require.Equal(t, tc.wantLimit, limit)
				require.Equal(t, tc.wantOffset, offset)
				require.Equal(t, org, orgID)
				require.Equal(t, user, userID)
				return []domain.Event{{ID: uuid.New()}}, nil
			}}
			page, err := service.NewEventsService(repo).GetEvents(context.Background(), tc.limit, tc.offset, org, user)
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			require.Equal(t, tc.wantLimit, page.Limit)
			require.Equal(t, tc.wantOffset, page.Offset)
			require.Len(t, page.Events, 1)
		})
	}
	for _, tc := range []struct {
		name          string
		limit, offset *int
		org, user     uuid.UUID
	}{
		{"zero", numberPointer(0), nil, org, user}, {"negative", numberPointer(-1), nil, org, user}, {"over max", numberPointer(101), nil, org, user},
		{"negative offset", nil, numberPointer(-1), org, user}, {"missing org", nil, nil, uuid.Nil, user}, {"missing user", nil, nil, org, uuid.Nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.NewEventsService(&repositoryStub{}).GetEvents(context.Background(), tc.limit, tc.offset, tc.org, tc.user)
			require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
		})
	}
}

func TestUpdateEventRules(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name    string
		input   service.UpdateEventInput
		status  domain.EventStatus
		wantErr error
	}{
		{"name only", service.UpdateEventInput{Name: textPointer("  Changed  ")}, domain.EventDraft, nil},
		{"clear optional", service.UpdateEventInput{Description: textPointer(""), Category: textPointer(" ")}, domain.EventDraft, nil},
		{"empty name", service.UpdateEventInput{Name: textPointer(" ")}, domain.EventDraft, coreerrors.ErrInvalidArgument},
		{"long name", service.UpdateEventInput{Name: textPointer(strings.Repeat("я", 201))}, domain.EventDraft, coreerrors.ErrInvalidArgument},
		{"long description", service.UpdateEventInput{Description: textPointer(strings.Repeat("x", 10001))}, domain.EventDraft, coreerrors.ErrInvalidArgument},
		{"long category", service.UpdateEventInput{Category: textPointer(strings.Repeat("x", 101))}, domain.EventDraft, coreerrors.ErrInvalidArgument},
		{"NUL", service.UpdateEventInput{Description: textPointer("bad\x00text")}, domain.EventDraft, coreerrors.ErrInvalidArgument},
		{"published", service.UpdateEventInput{Name: textPointer("Changed")}, domain.EventPublished, coreerrors.ErrConflict},
		{"archived", service.UpdateEventInput{Name: textPointer("Changed")}, domain.EventArchived, coreerrors.ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := domain.NewEvent(org, "Original", "Keep description", "conference")
			original.Status = tc.status
			writes := 0
			repo := &repositoryStub{
				get: func(ctx context.Context, id, orgID, userID uuid.UUID) (domain.Event, error) {
					require.Equal(t, original.ID, id)
					require.Equal(t, org, orgID)
					require.Equal(t, user, userID)
					return original, nil
				},
				update: func(ctx context.Context, e domain.Event, userID uuid.UUID) (domain.Event, error) {
					writes++
					require.Equal(t, user, userID)
					return e, nil
				},
			}
			result, err := service.NewEventsService(repo).UpdateEvent(context.Background(), tc.input, original.ID, org, user)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Zero(t, writes)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, writes)
			require.Equal(t, original.ID, result.ID)
			require.Equal(t, original.OrganizerID, result.OrganizerID)
			if tc.input.Name == nil {
				require.Equal(t, original.Title, result.Title)
			} else {
				require.Equal(t, strings.TrimSpace(*tc.input.Name), result.Title)
			}
			if tc.input.Description == nil {
				require.Equal(t, original.Description, result.Description)
			} else {
				require.Equal(t, strings.TrimSpace(*tc.input.Description), result.Description)
			}
			if tc.input.Category == nil {
				require.Equal(t, original.Category, result.Category)
			} else {
				require.Equal(t, strings.TrimSpace(*tc.input.Category), result.Category)
			}
		})
	}
}

func TestEventCommandsRejectInvalidIDs(t *testing.T) {
	org, user, id := uuid.New(), uuid.New(), uuid.New()
	for _, ids := range [][3]uuid.UUID{{uuid.Nil, org, user}, {id, uuid.Nil, user}, {id, org, uuid.Nil}} {
		s := service.NewEventsService(&repositoryStub{})
		_, err := s.GetEvent(context.Background(), ids[0], ids[1], ids[2])
		require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
		_, err = s.UpdateEvent(context.Background(), service.UpdateEventInput{Name: textPointer("Name")}, ids[0], ids[1], ids[2])
		require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
		require.ErrorIs(t, s.DeleteEvent(context.Background(), ids[0], ids[1], ids[2]), coreerrors.ErrInvalidArgument)
	}
	_, err := service.NewEventsService(&repositoryStub{}).UpdateEvent(context.Background(), service.UpdateEventInput{}, id, org, user)
	require.ErrorIs(t, err, coreerrors.ErrInvalidArgument)
}

func TestRepositoryErrorsArePreserved(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	event := domain.NewEvent(org, "Name", "Description", "conference")
	for _, expected := range []error{coreerrors.ErrNotFound, coreerrors.ErrForbidden, coreerrors.ErrConflict, fmt.Errorf("database unavailable")} {
		wrapped := fmt.Errorf("repository: %w", expected)
		repo := &repositoryStub{
			get: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Event, error) {
				return domain.Event{}, wrapped
			},
			list:   func(context.Context, int, int, uuid.UUID, uuid.UUID) ([]domain.Event, error) { return nil, wrapped },
			delete: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error { return wrapped },
		}
		s := service.NewEventsService(repo)
		_, err := s.GetEvent(context.Background(), event.ID, org, user)
		require.ErrorIs(t, err, expected)
		_, err = s.GetEvents(context.Background(), nil, nil, org, user)
		require.ErrorIs(t, err, expected)
		_, err = s.UpdateEvent(context.Background(), service.UpdateEventInput{Name: textPointer("Change")}, event.ID, org, user)
		require.ErrorIs(t, err, expected)
		require.ErrorIs(t, s.DeleteEvent(context.Background(), event.ID, org, user), expected)
		repo.get = func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Event, error) { return event, nil }
		repo.update = func(context.Context, domain.Event, uuid.UUID) (domain.Event, error) { return domain.Event{}, wrapped }
		_, err = s.UpdateEvent(context.Background(), service.UpdateEventInput{Name: textPointer("Change")}, event.ID, org, user)
		require.ErrorIs(t, err, expected)
	}
}

func (r *repositoryStub) PublishEvent(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Event, error) {
	panic("unexpected PublishEvent call")
}
