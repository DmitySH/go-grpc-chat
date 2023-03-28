package chatroom

import (
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"github.com/DmitySH/go-grpc-chat/pkg/entity"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"sync"
)

type Room struct {
	mu           sync.RWMutex
	users        map[uuid.UUID]entity.User
	messageQueue chan entity.Message
}

func NewRoom() *Room {
	return &Room{
		users:        make(map[uuid.UUID]entity.User),
		messageQueue: make(chan entity.Message),
	}
}

func (r *Room) AddUser(user entity.User) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[user.ID] = user
}

func (r *Room) DeleteUser(user entity.User) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.users, user.ID)
}

func (r *Room) PushMessage(message entity.Message) {
	r.messageQueue <- entity.Message{
		Content: message.Content,
		From:    message.From,
	}
}

func (r *Room) StartDeliveringMessages() {
	go func() {
		for msg := range r.messageQueue {
			r.handleMessage(msg)
		}
		log.Println("room stopped")
	}()
}

func (r *Room) CloseIfEmpty() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.users) == 0 {
		close(r.messageQueue)
		return true
	}

	return false
}

func (r *Room) handleMessage(msg entity.Message) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, user := range r.users {
		err := user.OutputStream.Send(&chat.MessageResponse{
			Username: msg.From,
			Content:  msg.Content + "\n",
		})

		if err != nil {
			log.Printf("can't send message to %s: %v", user.Name, err)
			if s, ok := status.FromError(err); ok {
				if s.Code() == codes.Unavailable {
					delete(r.users, user.ID)
				}
			}
		}
	}
}
