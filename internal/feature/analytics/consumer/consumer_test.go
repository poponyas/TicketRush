package analytics_consumer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	consumer "github.com/poponyas/TicketRush/internal/feature/analytics/consumer"
	repository "github.com/poponyas/TicketRush/internal/feature/analytics/repository"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
)

type messageReaderStub struct {
	fetch  func(context.Context) (kafka.Message, error)
	commit func(context.Context, ...kafka.Message) error
}

func (r *messageReaderStub) FetchMessage(ctx context.Context) (kafka.Message, error) {
	return r.fetch(ctx)
}

func (r *messageReaderStub) CommitMessages(ctx context.Context, messages ...kafka.Message) error {
	return r.commit(ctx, messages...)
}

type bookingEventRepositoryStub struct {
	insert func(context.Context, []repository.BookingEvent) error
}

func (r *bookingEventRepositoryStub) InsertBatch(ctx context.Context, events []repository.BookingEvent) error {
	return r.insert(ctx, events)
}

func TestKafkaConsumerBatchesBySizeAndCommitsAfterInsert(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	messages, ids := kafkaBookingMessages(t, 4)
	var calls []string
	var inserted [][]repository.BookingEvent
	fetchCount, commitCount := 0, 0

	reader := &messageReaderStub{
		fetch: func(ctx context.Context) (kafka.Message, error) {
			require.Less(t, fetchCount, len(messages))
			message := messages[fetchCount]
			fetchCount++
			return message, nil
		},
		commit: func(ctx context.Context, batch ...kafka.Message) error {
			require.NoError(t, ctx.Err())
			require.Equal(t, messages[commitCount*2:commitCount*2+2], batch)
			calls = append(calls, "commit")
			commitCount++
			if commitCount == 2 {
				cancel()
			}
			return nil
		},
	}
	repo := &bookingEventRepositoryStub{insert: func(ctx context.Context, events []repository.BookingEvent) error {
		require.NoError(t, ctx.Err())
		calls = append(calls, "insert")
		inserted = append(inserted, append([]repository.BookingEvent(nil), events...))
		return nil
	}}
	c, err := consumer.NewKafkaConsumer(reader, repo, 2, time.Hour)
	require.NoError(t, err)
	require.NoError(t, c.Run(ctx))
	require.Equal(t, []string{"insert", "commit", "insert", "commit"}, calls)
	require.Len(t, inserted, 2)
	require.Equal(t, ids[0], inserted[0][0].MessageID)
	require.Equal(t, ids[1], inserted[0][1].MessageID)
	require.Equal(t, ids[2], inserted[1][0].MessageID)
	require.Equal(t, ids[3], inserted[1][1].MessageID)
}

func TestKafkaConsumerFlushesPartialBatchOnFixedDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	messages, ids := kafkaBookingMessages(t, 3)
	var deadlines []time.Time
	fetchCount, insertCount, commitCount := 0, 0, 0

	reader := &messageReaderStub{
		fetch: func(ctx context.Context) (kafka.Message, error) {
			deadline, ok := ctx.Deadline()
			require.True(t, ok)
			deadlines = append(deadlines, deadline)
			if fetchCount < len(messages) {
				message := messages[fetchCount]
				fetchCount++
				return message, nil
			}
			<-ctx.Done()
			return kafka.Message{}, ctx.Err()
		},
		commit: func(ctx context.Context, batch ...kafka.Message) error {
			require.NoError(t, ctx.Err())
			require.Equal(t, 1, insertCount)
			require.Equal(t, messages, batch)
			commitCount++
			cancel()
			return nil
		},
	}
	repo := &bookingEventRepositoryStub{insert: func(ctx context.Context, events []repository.BookingEvent) error {
		// The collection context expired, but the database gets the live parent.
		require.NoError(t, ctx.Err())
		require.Len(t, events, 3)
		for i := range events {
			require.Equal(t, ids[i], events[i].MessageID)
		}
		insertCount++
		return nil
	}}
	c, err := consumer.NewKafkaConsumer(reader, repo, 100, 20*time.Millisecond)
	require.NoError(t, err)
	require.NoError(t, c.Run(ctx))
	require.Equal(t, 1, commitCount)
	require.Len(t, deadlines, 4)
	require.True(t, deadlines[0].After(deadlines[1]))
	require.Equal(t, deadlines[1], deadlines[2])
	require.Equal(t, deadlines[1], deadlines[3])
}

func TestKafkaConsumerStopsWithoutCommittingFailedBatch(t *testing.T) {
	for _, stage := range []string{"fetch", "decode", "insert", "commit"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			messages, _ := kafkaBookingMessages(t, 2)
			failure := errors.New(stage + " failed")
			fetchCount, insertCount, commitCount := 0, 0, 0
			reader := &messageReaderStub{
				fetch: func(context.Context) (kafka.Message, error) {
					if stage == "fetch" && fetchCount == 1 {
						return kafka.Message{}, failure
					}
					require.Less(t, fetchCount, len(messages))
					message := messages[fetchCount]
					fetchCount++
					if stage == "decode" && fetchCount == 2 {
						message.Value = []byte(`{"id":`)
					}
					return message, nil
				},
				commit: func(context.Context, ...kafka.Message) error {
					commitCount++
					require.Equal(t, 1, insertCount)
					return failure
				},
			}
			repo := &bookingEventRepositoryStub{insert: func(context.Context, []repository.BookingEvent) error {
				insertCount++
				if stage == "insert" {
					return failure
				}
				return nil
			}}
			c, err := consumer.NewKafkaConsumer(reader, repo, 2, time.Second)
			require.NoError(t, err)
			err = c.Run(ctx)
			require.Error(t, err)
			if stage != "decode" {
				require.ErrorIs(t, err, failure)
			}
			if stage == "commit" {
				require.Equal(t, 1, commitCount)
			} else {
				require.Zero(t, commitCount)
			}
			if stage == "fetch" || stage == "decode" {
				require.Zero(t, insertCount)
			}
		})
	}
}

func TestKafkaConsumerCancellationLeavesMessagesUncommitted(t *testing.T) {
	for _, partial := range []bool{false, true} {
		name := "waiting for first message"
		if partial {
			name = "partially collected batch"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			messages, _ := kafkaBookingMessages(t, 1)
			fetched := false
			reader := &messageReaderStub{
				fetch: func(ctx context.Context) (kafka.Message, error) {
					if partial && !fetched {
						fetched = true
						return messages[0], nil
					}
					cancel()
					return kafka.Message{}, ctx.Err()
				},
				commit: func(context.Context, ...kafka.Message) error {
					t.Fatal("cancelled batch must not be committed")
					return nil
				},
			}
			repo := &bookingEventRepositoryStub{insert: func(context.Context, []repository.BookingEvent) error {
				t.Fatal("cancelled batch must not be inserted")
				return nil
			}}
			c, err := consumer.NewKafkaConsumer(reader, repo, 100, time.Second)
			require.NoError(t, err)
			require.NoError(t, c.Run(ctx))
		})
	}
}

func TestNewKafkaConsumerValidatesDependenciesAndLimits(t *testing.T) {
	reader := &messageReaderStub{}
	repo := &bookingEventRepositoryStub{}
	for _, tc := range []struct {
		name     string
		reader   consumer.MessageReader
		repo     consumer.BookingEventRepository
		size     int
		interval time.Duration
	}{
		{"missing reader", nil, repo, 100, time.Second},
		{"missing repository", reader, nil, 100, time.Second},
		{"zero batch size", reader, repo, 0, time.Second},
		{"negative batch size", reader, repo, -1, time.Second},
		{"zero interval", reader, repo, 100, 0},
		{"negative interval", reader, repo, 100, -time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := consumer.NewKafkaConsumer(tc.reader, tc.repo, tc.size, tc.interval)
			require.Error(t, err)
			require.Nil(t, c)
		})
	}
}

func kafkaBookingMessages(t *testing.T, count int) ([]kafka.Message, []uuid.UUID) {
	t.Helper()
	messages := make([]kafka.Message, 0, count)
	ids := make([]uuid.UUID, 0, count)
	for i := 0; i < count; i++ {
		message, payload := validBookingMessage()
		messages = append(messages, kafka.Message{
			Topic: "ticketrush.bookings", Partition: i % 2, Offset: int64(i),
			Value: encodeBookingMessage(t, message, payload),
		})
		ids = append(ids, message.ID)
	}
	return messages, ids
}
