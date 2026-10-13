package core_transport

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func GetIDInPath(r *http.Request, key string) (uuid.UUID, error) {
	id := r.PathValue(key)
	idUU, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, fmt.Errorf(
			"%w: invalid path parameter %q: %v",
			core_errors.ErrInvalidArgument, key, err,
		)
	}

	return idUU, nil

}
