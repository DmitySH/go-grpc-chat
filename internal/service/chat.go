package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"github.com/DmitySH/go-grpc-chat/internal/chatroom"
	"github.com/DmitySH/go-grpc-chat/pkg/entity"
	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	"io"
	"log"
	"sync"
	"time"
)

var ErrUserStoppedChatting = errors.New("user stopped chatting")

type ChatService struct {
	chat.UnimplementedChatServer
	rooms  map[string]*chatroom.Room
	stopCh chan struct{}
	mu     sync.RWMutex
}

func NewChatService() *ChatService {
	service := &ChatService{
		rooms:  make(map[string]*chatroom.Room),
		stopCh: make(chan struct{}, 1),
	}
	service.checkEmptyRooms()

	return service
}

func (s *ChatService) DoChatting(msgStream chat.Chat_DoChattingServer) error {
	md, mdErr := checkMetadata(msgStream.Context())

	if mdErr != nil {
		return fmt.Errorf("incorrect metadata: %w", mdErr)
	}

	username := md.Get("username")[0]
	roomName := md.Get("room")[0]

	log.Println("user", username, "connected to room", roomName)

	user := entity.User{
		ID:            uuid.New(),
		Name:          username,
		MessageStream: msgStream,
	}

	room := s.getOrCreateRoom(roomName)
	room.AddUser(user)
	defer func() {
		room.DeleteUser(user)
		log.Println("user", user.Name, "disconnected")
	}()

	for {
		err := s.handleInputMessage(user, room)
		if errors.Is(err, ErrUserStoppedChatting) {
			return nil
		}

		if err != nil {
			return err
		}
	}
}

func (s *ChatService) getOrCreateRoom(roomName string) *chatroom.Room {
	s.mu.Lock()
	if _, ok := s.rooms[roomName]; !ok {
		s.rooms[roomName] = chatroom.NewRoom(roomName)
		s.rooms[roomName].StartDeliveringMessages()
	}
	room := s.rooms[roomName]
	s.mu.Unlock()

	return room
}

func (s *ChatService) handleInputMessage(user entity.User, room *chatroom.Room) error {
	in, err := user.MessageStream.Recv()

	if err == io.EOF {
		return ErrUserStoppedChatting
	}

	if err != nil {
		return fmt.Errorf("can't receive message: %w", err)
	}

	room.PushMessage(entity.Message{
		Content: in.Content,
		From:    user.Name,
	})
	log.Printf("room %s: %s said %s", room.Name, user.Name, in.Content)

	return nil
}

func (s *ChatService) Stop() {
	s.stopCh <- struct{}{}
}

func (s *ChatService) checkEmptyRooms() {
	ticker := time.Tick(time.Minute * 10)

	go func() {
		for {
			select {
			case <-ticker:
				s.deleteEmptyRooms()
			case <-s.stopCh:
				s.deleteEmptyRooms()
				return
			}
		}
	}()
}

func (s *ChatService) deleteEmptyRooms() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for roomName, room := range s.rooms {
		if ok := room.CloseIfEmpty(); ok {
			delete(s.rooms, roomName)
			log.Printf("room %s closed", roomName)
		}
	}
}

func checkMetadata(ctx context.Context) (metadata.MD, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no metadata in request")
	}
	if len(md.Get("username")) == 0 {
		return nil, fmt.Errorf("no username in metadata")
	}
	if len(md.Get("room")) == 0 {
		return nil, fmt.Errorf("no room in metadata")
	}

	return md, nil
}
