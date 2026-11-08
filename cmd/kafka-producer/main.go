package main

import (
	"context"

	"github.com/segmentio/kafka-go"
)

func main() {
	writer := &kafka.Writer{
		Addr:         kafka.TCP("127.0.0.1:9092"),
		Topic:        "ticketrush.playground",
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireAll,
	}

	defer writer.Close()

	err := writer.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte("demo-booking-1"),
		Value: []byte("hello from Go"),
	})

	if err != nil {
		return
	}
}
