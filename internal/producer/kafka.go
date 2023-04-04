package producer

import (
	"context"
	"errors"
	"fmt"
	"github.com/DmitySH/go-grpc-chat/pkg/config"
	"github.com/segmentio/kafka-go"
	"log"
	"time"
)

const maxKafkaRetries = 3

func WriteMessagesToKafka(messages []kafka.Message, config config.KafkaConfig) {
	writer := kafka.Writer{
		Addr:                   config.Addr,
		Topic:                  config.Topic,
		AllowAutoTopicCreation: config.AllowAutoTopicCreation,
	}
	defer func() {
		if err := writer.Close(); err != nil {
			log.Println("can't close kafka writer:", err)
		}
	}()

	if err := attemptSendMessages(&writer, messages); err != nil {
		log.Println(err)
	}
}

func attemptSendMessages(writer *kafka.Writer, messages []kafka.Message) error {
	var err error
	for i := 0; i < maxKafkaRetries; i++ {
		err = writeMessages(writer, messages)
		if errors.Is(err, kafka.LeaderNotAvailable) || errors.Is(err, context.DeadlineExceeded) {
			time.Sleep(time.Millisecond * 250)
			continue
		}

		if err != nil {
			return fmt.Errorf("can't write messages to kafka: %w", err)
		}
		return nil
	}

	return fmt.Errorf("too many attempts to write to kafka failed: %w", err)
}

func writeMessages(w *kafka.Writer, messages []kafka.Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := w.WriteMessages(ctx, messages...)

	return err
}
