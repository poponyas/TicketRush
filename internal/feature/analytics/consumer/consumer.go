package analytics_consumer

import (
	"context"
	"errors"
	"fmt"
	"time"

	analytics_repository "github.com/poponyas/TicketRush/internal/feature/analytics/repository"
	"github.com/segmentio/kafka-go"
)

type MessageReader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, messages ...kafka.Message) error
}

type BookingEventRepository interface {
	InsertBatch(ctx context.Context, events []analytics_repository.BookingEvent) error
}

type KafkaConsumer struct {
	kafkaConsumer       MessageReader
	analyticsRepository BookingEventRepository
	batchSize           int
	interval            time.Duration
}

func NewKafkaConsumer(
	kafkaConsumer MessageReader,
	analyticsRepository BookingEventRepository,
	batchSize int,
	interval time.Duration,
) (*KafkaConsumer, error) {
	if kafkaConsumer == nil {
		return nil, fmt.Errorf("kafka reader must not be nil")
	}
	if analyticsRepository == nil {
		return nil, fmt.Errorf("analytics repository must not be nil")
	}
	if batchSize <= 0 {
		return nil, fmt.Errorf("batch size must be positive")
	}
	if interval <= 0 {
		return nil, fmt.Errorf("batch interval must be positive")
	}

	return &KafkaConsumer{
		kafkaConsumer:       kafkaConsumer,
		analyticsRepository: analyticsRepository,
		batchSize:           batchSize,
		interval:            interval,
	}, nil
}

func (c *KafkaConsumer) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		events, messages, err := c.collectBatch(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if ctx.Err() != nil {
			return nil
		}

		if err := c.analyticsRepository.InsertBatch(ctx, events); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("insert booking events batch: %w", err)
		}

		commitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = c.kafkaConsumer.CommitMessages(commitCtx, messages...)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("commit booking event messages: %w", err)
		}
	}
	return nil
}

func (c *KafkaConsumer) collectBatch(ctx context.Context) ([]analytics_repository.BookingEvent, []kafka.Message, error) {
	message, err := c.kafkaConsumer.FetchMessage(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch message: %w", err)
	}

	batchCtx, cancel := context.WithTimeout(ctx, c.interval)
	defer cancel()
	events := make([]analytics_repository.BookingEvent, 0, c.batchSize)
	messages := make([]kafka.Message, 0, c.batchSize)

	for {
		bookingEvent, err := DecodeBookingEvent(message.Value)
		if err != nil {
			return nil, nil, fmt.Errorf("decode message %s/%d offset %d: %w", message.Topic, message.Partition, message.Offset, err)
		}
		events = append(events, bookingEvent)
		messages = append(messages, message)
		if len(events) == c.batchSize || batchCtx.Err() != nil {
			break
		}

		message, err = c.kafkaConsumer.FetchMessage(batchCtx)
		if err != nil {
			if ctx.Err() != nil {
				return nil, nil, ctx.Err()
			}
			if errors.Is(err, context.DeadlineExceeded) && errors.Is(batchCtx.Err(), context.DeadlineExceeded) {
				break
			}
			return nil, nil, fmt.Errorf("fetch message: %w", err)
		}
	}

	return events, messages, nil
}
