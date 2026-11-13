package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	core_clickhouse "github.com/poponyas/TicketRush/internal/core/repository/clickhouse"
	analytics_consumer "github.com/poponyas/TicketRush/internal/feature/analytics/consumer"
	analytics_repository "github.com/poponyas/TicketRush/internal/feature/analytics/repository"
	"github.com/segmentio/kafka-go"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load env: %w", err)
	}

	cfg, err := core_clickhouse.NewConfig()
	if err != nil {
		return fmt.Errorf("configure clickhouse: %w", err)
	}

	conn, err := core_clickhouse.NewClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect clickhouse: %w", err)
	}

	defer func() {
		if err := conn.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "close clickhouse: %v\n", err)
		}
	}()

	fmt.Println("ClickHouse connected")

	brokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(brokers) == 1 && brokers[0] == "" {
		brokers = []string{"127.0.0.1:9092"}
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          "ticketrush.bookings",
		GroupID:        "ticketrush-booking-analytics-v1",
		StartOffset:    kafka.FirstOffset,
		CommitInterval: 0,
	})
	defer func() {
		if err := reader.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "close kafka reader: %v\n", err)
		}
	}()

	repository := analytics_repository.NewAnalytics(conn, cfg.Timeout)
	consumer, err := analytics_consumer.NewKafkaConsumer(reader, repository, 100, time.Second)
	if err != nil {
		return fmt.Errorf("configure analytics consumer: %w", err)
	}

	fmt.Println("Booking analytics consumer started; press Ctrl+C to stop")
	return consumer.Run(ctx)
}
