package outbox_publisher

import "github.com/segmentio/kafka-go"

type KafkaPublisher struct {
	kafkaWriter *kafka.Writer
}

func NewKafkaPublisher(writer *kafka.Writer) *KafkaPublisher {
	return &KafkaPublisher{
		kafkaWriter: writer,
	}
}
