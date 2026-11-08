package outbox_repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

func (r *OutboxRepository) MarkPublished(
	ctx context.Context,
	id uuid.UUID,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.Pool.OpTimeout)
	defer cancel()

	const query = `
	UPDATE outbox_events
	SET published_at = NOW()
	WHERE id = $1 AND published_at IS NULL;
	`

	if _, err := r.Pool.Pool.Exec(ctx, query, id); err != nil {
		return fmt.Errorf("exec: %w", err)
	}

	return nil
}
