package producer

import (
	"context"
	"errors"
	"fmt"
	"github.com/segmentio/kafka-go"
	"log"
	"time"
)

const (
	maxKafkaRetries         = 3
	defaultWaitTime         = time.Second * 5
	defaultBetweenRetryTime = time.Millisecond * 250
)

func AttemptSendMessages(writer *kafka.Writer, messages []kafka.Message) error {
	var err error
	for i := 0; i < maxKafkaRetries; i++ {
		err = writeMessages(writer, messages)
		if errors.Is(err, kafka.LeaderNotAvailable) || errors.Is(err, context.DeadlineExceeded) {
			time.Sleep(defaultBetweenRetryTime)
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
	ctx, cancel := context.WithTimeout(context.Background(), defaultWaitTime)
	defer cancel()

	switch err := w.WriteMessages(ctx, messages...).(type) {
	case nil:
		return nil
	case kafka.WriteErrors:
		for i := range messages {
			if err[i] != nil {
				log.Println("message with key", string(messages[i].Key), "was not sent")
			}
		}
		return err
	default:
		return err
	}
}
