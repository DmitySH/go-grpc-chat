package chatroom

import (
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"log"
	"sync"
)

type Room struct {
	mu           sync.RWMutex
	currentID    int64
	users        map[int64]User
	messageQueue chan Message
}

func NewRoom() *Room {
	return &Room{
		users:        make(map[int64]User),
		messageQueue: make(chan Message),
	}
}

func (r *Room) AddUser(user User) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[r.currentID] = user
	r.currentID++
}

func (r *Room) PushMessage(message Message) {
	r.messageQueue <- Message{
		Content: message.Content,
		From:    message.From,
	}
}

func (r *Room) StartDeliveringMessages() {
	go func() {
		for msg := range r.messageQueue {
			r.mu.RLock()
			for _, user := range r.users {
				err := user.OutputStream.Send(&chat.MessageResponse{
					Username: msg.From,
					Content:  msg.Content + "\n",
				})
				if err != nil {
					log.Println("can't send message to:", err)
				}
			}
			r.mu.RUnlock()
		}
	}()
}
