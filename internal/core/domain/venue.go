package domain

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

type Venue struct {
	ID      uuid.UUID `db:"id"`
	Name    string    `db:"name"`
	City    string    `db:"city"`
	Address string    `db:"address"`
}

func NewVenue(name, city, address string) Venue {
	id := uuid.New()
	return Venue{
		ID:      id,
		Name:    strings.TrimSpace(name),
		City:    strings.TrimSpace(city),
		Address: strings.TrimSpace(address),
	}
}

func (v Venue) Validate() error {
	if v.ID == uuid.Nil {
		return fmt.Errorf("%w: venue id must not contain a nil", core_errors.ErrInvalidArgument)
	}

	uName := utf8.RuneCountInString(v.Name)
	if uName > 200 || uName < 1 || strings.TrimSpace(v.Name) == "" {
		return fmt.Errorf("%w: venue name must contain 1 to 200 characters", core_errors.ErrInvalidArgument)
	}

	uCity := utf8.RuneCountInString(v.City)
	if uCity > 100 || uCity < 1 || strings.TrimSpace(v.City) == "" {
		return fmt.Errorf("%w: venue city must must contain 1 to 100 characters", core_errors.ErrInvalidArgument)
	}

	uAddress := utf8.RuneCountInString(v.Address)
	if uAddress > 500 || uAddress < 1 || strings.TrimSpace(v.Address) == "" {
		return fmt.Errorf("%w: venue address must must contain 1 to 500 characters", core_errors.ErrInvalidArgument)
	}

	if strings.ContainsRune(v.Name, '\x00') || strings.ContainsRune(v.City, '\x00') || strings.ContainsRune(v.Address, '\x00') {
		return fmt.Errorf("%w: event text must not contain a null character", core_errors.ErrInvalidArgument)
	}

	return nil
}
