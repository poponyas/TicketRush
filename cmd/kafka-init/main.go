// kafka-init creates the application topic idempotently using the advertised broker.
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/segmentio/kafka-go"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		return err
	}
	broker := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")[0]
	if broker == "" {
		broker = "127.0.0.1:9092"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := kafka.DialContext(ctx, "tcp", broker)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	controller, err := conn.Controller()
	if err != nil {
		return err
	}
	admin, err := kafka.DialContext(ctx, "tcp", net.JoinHostPort(controller.Host, fmt.Sprint(controller.Port)))
	if err != nil {
		return err
	}
	defer admin.Close()
	_ = admin.SetDeadline(time.Now().Add(10 * time.Second))
	err = admin.CreateTopics(kafka.TopicConfig{Topic: "ticketrush.bookings", NumPartitions: 1, ReplicationFactor: 1})
	if errors.Is(err, kafka.TopicAlreadyExists) {
		return nil
	}
	return err
}
