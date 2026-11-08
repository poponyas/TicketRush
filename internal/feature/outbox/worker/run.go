package outbox_worker

import (
	"context"
	"time"

	"go.uber.org/zap"
)

func (w *OutboxWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		if ctx.Err() != nil {
			return
		}

		if err := w.ProcessBatch(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}

			w.logger.Error("process outbox batch", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
