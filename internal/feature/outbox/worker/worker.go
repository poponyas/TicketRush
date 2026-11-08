package outbox_worker

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/domain"
	core_logger "github.com/poponyas/TicketRush/internal/core/logger"
)

type Repository interface {
	GetPendingEvents(
		ctx context.Context,
		limit int,
	) ([]domain.OutboxEvent, error)

	MarkPublished(ctx context.Context, id uuid.UUID) error
}

type Publisher interface {
	Publish(ctx context.Context, event domain.OutboxEvent) error
}

type OutboxWorker struct {
	outboxRepository Repository
	kafkaPublisher   Publisher
	batchSize        int
	interval         time.Duration
	logger           *core_logger.Logger
}

func NewOutboxWorker(
	outboxRepository Repository,
	kafkaPublisher Publisher,
	batchSize int,
	interval time.Duration,
	logger *core_logger.Logger,
) (*OutboxWorker, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("interval must be greater than zero")
	}

	if logger == nil || logger.Logger == nil {
		return nil, fmt.Errorf("logger can't be a nil pointer")
	}
	if batchSize <= 0 {
		return nil, fmt.Errorf(
			"batchSize must be greater than zero",
		)
	}
	return &OutboxWorker{
		outboxRepository: outboxRepository,
		kafkaPublisher:   kafkaPublisher,
		batchSize:        batchSize,
		interval:         interval,
		logger:           logger,
	}, nil
}

func (w *OutboxWorker) ProcessBatch(ctx context.Context) error {
	events, err := w.outboxRepository.GetPendingEvents(ctx, w.batchSize)
	if err != nil {
		return fmt.Errorf("worker get pending from outbox: %w", err)
	}

	for _, event := range events {
		if err := w.kafkaPublisher.Publish(ctx, event); err != nil {
			return fmt.Errorf("publish outbox event %s: %w", event.ID, err)
		}

		if err := w.outboxRepository.MarkPublished(ctx, event.ID); err != nil {
			return fmt.Errorf("mark is_published from worker: %w", err)
		}
	}

	return nil
}
