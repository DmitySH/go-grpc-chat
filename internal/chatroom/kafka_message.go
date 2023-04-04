package chatroom

import (
	"encoding/json"
	"github.com/DmitySH/go-grpc-chat/pkg/entity"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"log"
	"time"
)

type KafkaMessageValue struct {
	MessageUUID    uuid.UUID
	Timestamp      time.Time
	MessageContent string
	FromUserUUID   uuid.UUID
	ToUserUUID     uuid.UUID
}

func newKafkaMessage(msg entity.Message, user entity.User) kafka.Message {
	messageUUID := uuid.New()
	value := KafkaMessageValue{
		MessageUUID:    messageUUID,
		Timestamp:      time.Now(),
		MessageContent: msg.Content,
		FromUserUUID:   msg.FromUUID,
		ToUserUUID:     user.ID,
	}

	jsonValue, err := json.Marshal(value)
	if err != nil {
		log.Printf("can't marshall message: %v", err)
	}

	return kafka.Message{
		Key:   []byte(messageUUID.String()),
		Value: jsonValue,
	}
}
