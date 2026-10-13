package core_transport

import (
	"fmt"
	"net/http"
	"strconv"

	core_errors "github.com/poponyas/TicketRush/internal/core/errors"
)

func GetQueryByURL(r *http.Request, key string) (*int, error) {
	values, present := r.URL.Query()[key]
	if !present {
		return nil, nil
	}
	if len(values) != 1 || values[0] == "" {
		return nil, fmt.Errorf("%w: query parameter %q must contain one integer", core_errors.ErrInvalidArgument, key)
	}

	queryInt, err := strconv.Atoi(values[0])
	if err != nil {
		return nil, fmt.Errorf("%w: query parameter %q: %v", core_errors.ErrInvalidArgument, key, err)
	}

	return &queryInt, nil
}
