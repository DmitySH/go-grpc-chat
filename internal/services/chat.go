package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"github.com/DmitySH/go-grpc-chat/internal/chatroom"
	"github.com/DmitySH/go-grpc-chat/pkg/config"
	"github.com/DmitySH/go-grpc-chat/pkg/cryptotransfer"
	"github.com/DmitySH/go-grpc-chat/pkg/entity"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"io"
	"log"
	"sync"
)

const cryptoBits = 2048

var ErrUserDisconnected = errors.New("user disconnected")
var ErrUnsafeChat = errors.New("can't use cipher for messages")

type ChatService struct {
	chat.UnimplementedChatServer
	rooms       map[string]*chatroom.Room
	roomsMu     sync.Mutex
	kafkaConfig config.KafkaConfig
}

func NewChatService(kafkaConfig config.KafkaConfig) *ChatService {
	service := &ChatService{
		rooms:       make(map[string]*chatroom.Room),
		kafkaConfig: kafkaConfig,
	}

	return service
}

// TODO: ошибки, отправляемые наружу сделать бизнесовыми.

func (s *ChatService) DoChatting(msgStream chat.Chat_DoChattingServer) error {
	md, mdErr := checkMetadata(msgStream.Context())
	if mdErr != nil {
		return mdErr
	}

	serverKeyPair, generateKeyErr := cryptotransfer.GenerateKeyPair(cryptoBits)
	if generateKeyErr != nil {
		log.Println("can't generate key pair:", generateKeyErr)
		return ErrUnsafeChat
	}

	username := md.Get("username")[0]
	roomName := md.Get("room")[0]

	user := entity.User{
		ID:               uuid.New(),
		Name:             username,
		MessageStream:    msgStream,
		ServerPrivateKey: serverKeyPair,
	}

	room, addUserErr := s.addUserToRoom(roomName, user)
	if addUserErr != nil {
		log.Printf("can't add user to room %s: %v\n", room.Name, addUserErr)
		return fmt.Errorf("can't add user to room %s: %w", room.Name, addUserErr)
	}

	if sendMdErr := user.MessageStream.SendHeader(s.prepareUserMeta(user)); sendMdErr != nil {
		log.Printf("can't send header metadata for user %s: %v\n", user.Name, sendMdErr)
		return fmt.Errorf("can't add user to room %s: %w", user.Name, sendMdErr)
	}

	log.Println("user", username, "connected to room", roomName)
	room.PushMessage(entity.Message{
		Content:  fmt.Sprintf("user %s connected", username),
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
		s.rooms[roomName] = chatroom.NewRoom(roomName, s.kafkaConfig)
		s.rooms[roomName].StartDeliveringMessages()

		log.Println("room", roomName, "created")
	}
	room := s.rooms[roomName]

	return room
}

func (s *ChatService) prepareUserMeta(user entity.User) metadata.MD {
	encodedPublicKey := cryptotransfer.EncodePublicKeyToBase64(&user.ServerPrivateKey.PublicKey)

	return metadata.New(map[string]string{
		"uuid":       user.ID.String(),
		"cipher_key": encodedPublicKey,
	})
}

func (s *ChatService) disconnectUser(user entity.User, room *chatroom.Room) {
	room.DeleteUser(user.ID)
	log.Println("user", user.Name, "disconnected")

	if ok := s.deleteRoomIfEmpty(room); ok {
		log.Println("room", room.Name, "deleted")
		return
	}

	room.PushMessage(entity.Message{
		Content:  fmt.Sprintf("user %s disconnected", user.Name),
		FromName: fmt.Sprintf("room %s", room.Name),
		FromUUID: user.ID,
	})
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

	message, decryptErr := cryptotransfer.DecryptRSAMessage(in.Content, user.ServerPrivateKey)
	if decryptErr != nil {
		return fmt.Errorf("can't decrypt message: %w", decryptErr)
	}

	room.PushMessage(entity.Message{
		Content:  message,
		FromName: user.Name,
		FromUUID: user.ID,
	})
	log.Printf("room %s: %s said %s", room.Name, user.Name, message)

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
