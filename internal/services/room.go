package services

import (
	"context"
	"fmt"
	"github.com/DmitySH/go-grpc-chat/internal/entity"
	"github.com/DmitySH/go-grpc-chat/pkg/api/chat"
	"github.com/DmitySH/go-grpc-chat/pkg/cryptotransfer"
	"github.com/google/uuid"
	"log"
	"sync"
	"time"
)

const (
	producerTimeout = time.Second * 5
)

type Producer interface {
	Close() error
	SendMessages(ctx context.Context, messages []entity.BrokerLoggingMessage) error
}

type Room struct {
	Name         string
	usersMu      sync.RWMutex
	users        map[uuid.UUID]entity.User
	messageQueue chan entity.Message
	closed       bool
	producer     Producer
}

func NewRoom(name string, producer Producer) *Room {
	return &Room{
		Name:         name,
		users:        make(map[uuid.UUID]entity.User),
		messageQueue: make(chan entity.Message),
		producer:     producer,
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
		if err := r.producer.Close(); err != nil {
			log.Println("can't close kafka writer in room", r.Name)
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

	kafkaMessages := make([]entity.BrokerLoggingMessage, 0, len(r.users))

	for _, user := range r.users {
		err := encryptAndSendMessage(msg, user)
		if err != nil {
			log.Printf("can't send message to %s: %v", user.Name, err)
		} else {
			kafkaMessages = append(kafkaMessages, newBrokerLoggingMessage(msg, user))
		}
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), producerTimeout)
		defer cancel()

		err := r.producer.SendMessages(ctx, kafkaMessages)
		if err != nil {
			log.Println("can't send messages to broker", err)
		}
	}()
}

func encryptAndSendMessage(msg entity.Message, user entity.User) error {
	cipherMessage, encryptErr := cryptotransfer.EncryptRSAMessage(msg.Content+"\n", user.ClientPublicKey)
	if encryptErr != nil {
		return fmt.Errorf("can't encrypt message: %w", encryptErr)
	}

	sendErr := user.MessageStream.Send(&chat.MessageResponse{
		FromName: msg.FromName,
		Content:  cipherMessage,
		FromUuid: msg.FromUUID.String(),
	})

	return sendErr
}

func newBrokerLoggingMessage(msg entity.Message, user entity.User) entity.BrokerLoggingMessage {
	messageUUID := uuid.New()

	return entity.BrokerLoggingMessage{
		MessageUUID:    messageUUID,
		Timestamp:      time.Now(),
		MessageContent: msg.Content,
		FromUserUUID:   msg.FromUUID,
		ToUserUUID:     user.ID,
	}
}
