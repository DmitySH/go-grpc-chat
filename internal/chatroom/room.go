package chatroom

import (
	"fmt"
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"github.com/DmitySH/go-grpc-chat/internal/producer"
	"github.com/DmitySH/go-grpc-chat/pkg/config"
	"github.com/DmitySH/go-grpc-chat/pkg/entity"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"log"
	"sync"
)

type Room struct {
	Name         string
	usersMu      sync.RWMutex
	users        map[uuid.UUID]entity.User
	messageQueue chan entity.Message
	closed       bool
	kafkaConfig  config.KafkaConfig
}

func NewRoom(name string, kafkaConfig config.KafkaConfig) *Room {
	return &Room{
		Name:         name,
		users:        make(map[uuid.UUID]entity.User),
		messageQueue: make(chan entity.Message),
		kafkaConfig:  kafkaConfig,
	}
}

func (r *Room) AddUser(user entity.User) error {
	r.usersMu.Lock()
	defer r.usersMu.Unlock()
	if r.closed {
		return fmt.Errorf("room %s is closed", r.Name)
	}

	r.users[user.ID] = user

	return nil
}

func (r *Room) DeleteUser(userID uuid.UUID) {
	r.usersMu.RLock()
	defer r.usersMu.RUnlock()
	delete(r.users, userID)
}

func (r *Room) PushMessage(message entity.Message) {
	r.messageQueue <- message
}

func (r *Room) StartDeliveringMessages() {
	go func() {
		for msg := range r.messageQueue {
			r.sendMessageToAllUsers(msg)
		}
	}()
}

func (r *Room) CloseIfEmpty() bool {
	r.usersMu.Lock()
	defer r.usersMu.Unlock()

	if len(r.users) == 0 {
		close(r.messageQueue)
		r.closed = true

		return true
	}

	return false
}

func (r *Room) sendMessageToAllUsers(msg entity.Message) {
	r.usersMu.RLock()
	defer r.usersMu.RUnlock()

	kafkaMessages := make([]kafka.Message, 0, len(r.users))

	for _, user := range r.users {
		err := user.MessageStream.Send(&chat.MessageResponse{
			Username: msg.FromName,
			Content:  msg.Content + "\n",
			FromUuid: msg.FromUUID.String(),
		})

		if err != nil {
			log.Printf("can't send message to %s: %v", user.Name, err)
		} else {
			kafkaMessages = append(kafkaMessages, newKafkaMessage(msg, user))
		}
	}

	go producer.WriteMessagesToKafka(kafkaMessages, r.kafkaConfig)
}
