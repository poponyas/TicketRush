package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

type EventStatus string

const (
	EventDraft     EventStatus = "draft"
	EventPublished EventStatus = "published"
	EventArchived  EventStatus = "archived"
)

// Event describes the product. Dates and inventory belong to Session and TicketType.
type Event struct {
	ID          uuid.UUID   `db:"id"`
	OrganizerID uuid.UUID   `db:"organizer_id"`
	Title       string      `db:"name"`
	Description string      `db:"description"`
	Category    string      `db:"category"`
	PosterKey   string      `db:"poster_key"`
	Status      EventStatus `db:"status"`
	CreatedAt   time.Time   `db:"created_at"`
	UpdatedAt   time.Time   `db:"updated_at"`
}

func NewEvent(organizerID uuid.UUID, title, description, category string) Event {
	return Event{ID: uuid.New(), OrganizerID: organizerID,
		Title: strings.TrimSpace(title), Description: strings.TrimSpace(description),
		Category: strings.TrimSpace(category), Status: EventDraft}
}
func (e Event) Validate() error {
	if e.ID == uuid.Nil || e.OrganizerID == uuid.Nil {
		return fmt.Errorf("%w: event and organizer IDs are required", coreerrors.ErrInvalidArgument)
	}
	if strings.ContainsRune(e.Title, '\x00') || strings.ContainsRune(e.Description, '\x00') || strings.ContainsRune(e.Category, '\x00') {
		return fmt.Errorf("%w: event text must not contain a null character", coreerrors.ErrInvalidArgument)
	}
	if strings.TrimSpace(e.Title) == "" || utf8.RuneCountInString(e.Title) > 200 {
		return fmt.Errorf("%w: name must contain 1 to 200 characters", coreerrors.ErrInvalidArgument)
	}
	if utf8.RuneCountInString(e.Description) > 10000 || utf8.RuneCountInString(e.Category) > 100 {
		return fmt.Errorf("%w: description or category is too long", coreerrors.ErrInvalidArgument)
	}
	if e.Status != EventDraft && e.Status != EventPublished && e.Status != EventArchived {
		return fmt.Errorf("%w: invalid event status", coreerrors.ErrInvalidArgument)
	}
	return nil
}
