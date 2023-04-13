package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DmitySH/go-grpc-chat/internal/entity"
	"github.com/segmentio/kafka-go"
	"log"
	"net"
	"time"
)

const (
	maxRetries       = 3
	betweenRetryTime = time.Millisecond * 250
)

type KafkaBroker struct {
	writer *kafka.Writer
}

func NewKafkaBroker(addr net.Addr, topic string, allowAutoTopicCreation bool) *KafkaBroker {
	return &KafkaBroker{writer: &kafka.Writer{
		Addr:                   addr,
		Topic:                  topic,
		AllowAutoTopicCreation: allowAutoTopicCreation,
	}}
}

func (k *KafkaBroker) SendMessages(ctx context.Context, messages []entity.BrokerLoggingMessage) error {
	kafkaMessages, convertErr := convertMessagesToKafka(messages)
	if convertErr != nil {
		return fmt.Errorf("can't send messages to kafka: %w", convertErr)
	}

	var err error
	for i := 0; i < maxRetries; i++ {
		err = k.writeMessages(ctx, kafkaMessages)
		if errors.Is(err, kafka.LeaderNotAvailable) || errors.Is(err, context.DeadlineExceeded) {
			time.Sleep(betweenRetryTime)
			continue
		}

		if err != nil {
			return fmt.Errorf("can't write messages to kafka: %w", err)
		}
		return nil
	}

	return fmt.Errorf("too many attempts to write to kafka failed: %w", err)
}

func convertMessagesToKafka(messages []entity.BrokerLoggingMessage) ([]kafka.Message, error) {
	kafkaMessages := make([]kafka.Message, len(messages))

	for i, msg := range messages {
		jsonMsg, err := json.Marshal(msg)
		if err != nil {
			return nil, fmt.Errorf("can't marshall message: %w", err)
		}

		kafkaMessages[i] = kafka.Message{
			Key:   []byte(msg.MessageUUID.String()),
			Value: jsonMsg,
		}
	}

	return kafkaMessages, nil
}

func (k *KafkaBroker) writeMessages(ctx context.Context, messages []kafka.Message) error {
	switch err := k.writer.WriteMessages(ctx, messages...).(type) {
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

func (k *KafkaBroker) Close() error {
	err := k.writer.Close()
	return err
}
