package service

import (
	"fmt"
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"github.com/DmitySH/go-grpc-chat/internal/chatroom"
	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	"io"
	"log"
	"sync"
	"time"
)

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
	md, ok := metadata.FromIncomingContext(msgStream.Context())
	if !ok {
		return fmt.Errorf("no metadata in request")
	}

	if err := usernameAndRoomInMetadata(md); err != nil {
		return err
	}

	username := md.Get("username")[0]
	roomName := md.Get("room")[0]

	log.Println("user", username, "connected to room", roomName)

	s.mu.Lock()
	if _, ok = s.rooms[roomName]; !ok {
		s.rooms[roomName] = chatroom.NewRoom()
		s.rooms[roomName].StartDeliveringMessages()
	}
	room := s.rooms[roomName]
	s.mu.Unlock()

	user := chatroom.User{
		ID:           uuid.New(),
		Name:         username,
		OutputStream: msgStream,
	}
	room.AddUser(user)

	for {
		in, err := msgStream.Recv()
		if err == io.EOF {
			room.DeleteUser(user)

			log.Println("user", username, "disconnected")
			return nil
		}

		if err != nil {
			log.Println("error during chatting:", err)
			return err
		}

		room.PushMessage(chatroom.Message{
			Content: in.Content,
			From:    username,
		})
		log.Printf("room %s: %s said %s", roomName, username, in.Content)
	}
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

func usernameAndRoomInMetadata(md metadata.MD) error {
	if len(md.Get("username")) == 0 {
		return fmt.Errorf("no username in metadata")
	}
	if len(md.Get("room")) == 0 {
		return fmt.Errorf("no room in metadata")
	}

	return nil
}
