package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{"127.0.0.1:9092"},
		Topic:          "ticketrush.playground",
		GroupID:        "ticketrush-playground-v1",
		StartOffset:    kafka.FirstOffset,
		CommitInterval: 0,
	})

	defer func() {
		if err := reader.Close(); err != nil {
			log.Printf("close reader: %v", err)
		}
	}()

	log.Print("consumer started; press Ctrl+C to stop")

	for {
		// Ждём следующее сообщение.
		message, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			return fmt.Errorf("fetch message: %w", err)
		}

		// Пока обработкой считаем вывод сообщения.
		log.Printf(
			"topic=%s partition=%d offset=%d key=%q value=%q",
			message.Topic,
			message.Partition,
			message.Offset,
			message.Key,
			message.Value,
		)

		// Сохраняем прогресс после обработки.
		commitCtx, cancel := context.WithTimeout(
			ctx,
			5*time.Second,
		)

		err = reader.CommitMessages(commitCtx, message)
		cancel()

		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			return fmt.Errorf("commit message: %w", err)
		}
	}
}
