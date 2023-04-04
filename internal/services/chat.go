package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"github.com/DmitySH/go-grpc-chat/internal/chatroom"
	"github.com/DmitySH/go-grpc-chat/pkg/entity"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"io"
	"log"
	"sync"
)

var ErrUserDisconnected = errors.New("user disconnected")

type ChatService struct {
	chat.UnimplementedChatServer
	rooms   map[string]*chatroom.Room
	roomsMu sync.Mutex
}

func NewChatService() *ChatService {
	service := &ChatService{
		rooms: make(map[string]*chatroom.Room),
	}

	return service
}

func (s *ChatService) DoChatting(msgStream chat.Chat_DoChattingServer) error {
	md, mdErr := checkMetadata(msgStream.Context())

	if mdErr != nil {
		return mdErr
	}

	username := md.Get("username")[0]
	roomName := md.Get("room")[0]

	user := entity.User{
		ID:            uuid.New(),
		Name:          username,
		MessageStream: msgStream,
	}

	room, addUserErr := s.addUserToRoom(roomName, user)
	if addUserErr != nil {
		log.Printf("can't add user to room %s: %v\n", room.Name, addUserErr)
		return fmt.Errorf("can't add user to room %s: %w", room.Name, addUserErr)
	}

	sendMdErr := user.MessageStream.SendHeader(metadata.New(map[string]string{"uuid": user.ID.String()}))
	if sendMdErr != nil {
		log.Printf("can't send header metadata for user %s: %v\n", user.Name, sendMdErr)
		return fmt.Errorf("can't add user to room %s: %w", user.Name, sendMdErr)
	}

	log.Println("user", username, "connected to room", roomName)
	room.PushMessage(entity.Message{
		Content:  fmt.Sprintf("user %s connected to room %s", username, roomName),
		FromName: fmt.Sprintf("room %s", roomName),
		FromUUID: user.ID,
	})
	defer s.disconnectUser(user, room)

	for {
		err := s.handleInputMessage(user, room)
		if errors.Is(err, ErrUserDisconnected) {
			return nil
		}

		if err != nil {
			log.Println(err)
			return err
		}
	}
}

func (s *ChatService) addUserToRoom(roomName string, user entity.User) (*chatroom.Room, error) {
	s.roomsMu.Lock()
	defer s.roomsMu.Unlock()

	room := s.getOrCreateRoom(roomName)
	err := room.AddUser(user)
	if err != nil {
		return nil, err
	}

	return room, nil
}

func (s *ChatService) getOrCreateRoom(roomName string) *chatroom.Room {
	if _, ok := s.rooms[roomName]; !ok {
		s.rooms[roomName] = chatroom.NewRoom(roomName)
		s.rooms[roomName].StartDeliveringMessages()

		log.Println("room", roomName, "created")
	}
	room := s.rooms[roomName]

	return room
}

func (s *ChatService) disconnectUser(user entity.User, room *chatroom.Room) {
	room.DeleteUser(user.ID)
	log.Println("user", user.Name, "disconnected")

	if ok := s.deleteRoomIfEmpty(room); ok {
		log.Println("room", room.Name, "deleted")
	}
}

func (s *ChatService) deleteRoomIfEmpty(room *chatroom.Room) bool {
	s.roomsMu.Lock()
	defer s.roomsMu.Unlock()

	if ok := room.CloseIfEmpty(); ok {
		delete(s.rooms, room.Name)
		return true
	}

	return false
}

func (s *ChatService) handleInputMessage(user entity.User, room *chatroom.Room) error {
	in, err := user.MessageStream.Recv()

	if err == io.EOF {
		return ErrUserDisconnected
	}

	if st, ok := status.FromError(err); ok && st.Code() == codes.Canceled {
		return ErrUserDisconnected
	}

	if err != nil {
		return fmt.Errorf("can't receive message: %w", err)
	}

	room.PushMessage(entity.Message{
		Content:  in.Content,
		FromName: user.Name,
		FromUUID: user.ID,
	})
	log.Printf("room %s: %s said %s", room.Name, user.Name, in.Content)

	return nil
}

func checkMetadata(ctx context.Context) (metadata.MD, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.PermissionDenied, "no metadata")
	}
	if len(md.Get("username")) == 0 {
		return nil, status.Error(codes.PermissionDenied, "no username in metadata")
	}
	if len(md.Get("room")) == 0 {
		return nil, status.Error(codes.PermissionDenied, "no room in metadata")
	}

	return md, nil
}
