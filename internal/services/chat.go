package services

import (
	"errors"
	"fmt"
	"github.com/DmitySH/go-grpc-chat/internal/entity"
	"github.com/DmitySH/go-grpc-chat/pkg/api/chat"
	"github.com/DmitySH/go-grpc-chat/pkg/cryptotransfer"
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

type ProducerFactory interface {
	Create() Producer
}

type ChatService struct {
	chat.UnimplementedChatServer
	rooms           map[string]*Room
	roomsMu         sync.Mutex
	producerFactory ProducerFactory
}

func NewChatService(producerFactory ProducerFactory) *ChatService {
	service := &ChatService{
		rooms:           make(map[string]*Room),
		producerFactory: producerFactory,
	}

	return service
}

func (s *ChatService) DoChatting(msgStream chat.Chat_DoChattingServer) error {
	md, _ := metadata.FromIncomingContext(msgStream.Context())

	roomName := md.Get("room")[0]

	user, createUserErr := createChatUser(md, msgStream)
	if createUserErr != nil {
		log.Println("can't create user:", createUserErr)
		return status.Error(codes.Internal, "can't create user:"+createUserErr.Error())
	}

	room, addUserErr := s.addUserToRoom(roomName, user)
	if addUserErr != nil {
		log.Printf("can't add user to room %s: %v\n", room.Name, addUserErr)
		return status.Error(codes.Internal, "can't add user to room:"+addUserErr.Error())
	}
	defer s.disconnectUser(user.User, room)

	if sendMdErr := user.MessageStream.SendHeader(prepareMetaForClient(user)); sendMdErr != nil {
		log.Printf("can't send header metadata for user %s: %v\n", user.Name, sendMdErr)
		return status.Error(codes.Internal, "can't send header metadata:"+sendMdErr.Error())
	}

	if sendHandshakeErr := sendHandshake(user.MessageStream); sendHandshakeErr != nil {
		log.Println("can't send handshake to client:", sendHandshakeErr)
		return status.Error(codes.Internal, "can't send handshake to client:"+sendHandshakeErr.Error())
	}

	log.Println("user", user.Name, "connected to room", roomName)
	room.PushMessage(entity.Message{
		Content:  fmt.Sprintf("user %s connected", user.Name),
		FromName: fmt.Sprintf("room %s", roomName),
		FromUUID: user.ID,
		Type:     entity.UserConnected,
	})

	return s.handleInputMessages(user, room)
}

func createChatUser(md metadata.MD, msgStream chat.Chat_DoChattingServer) (entity.ChatUser, error) {
	username := md.Get("username")[0]
	encodedClientPublicKey := md.Get("cipher_key")[0]

	serverKeyPair, generateKeyErr := cryptotransfer.GenerateKeyPair(cryptoBits)
	if generateKeyErr != nil {
		log.Println("can't generate key pair:", generateKeyErr)
		return entity.ChatUser{}, fmt.Errorf("can't generate key pair: %w", generateKeyErr)
	}

	clientPublicKey, decodeKeyErr := cryptotransfer.DecodePublicKeyFromBase64(encodedClientPublicKey)
	if decodeKeyErr != nil {
		return entity.ChatUser{}, fmt.Errorf("can't decode client's key from base64: %w", decodeKeyErr)
	}

	user := entity.ChatUser{
		User:              entity.User{ID: uuid.New(), Name: username},
		MessageStream:     msgStream,
		ServerPrivateKey:  serverKeyPair,
		ClientPublicKey:   clientPublicKey,
		ReceivingMessages: new(bool),
	}

	return user, nil
}

func (s *ChatService) addUserToRoom(roomName string, user entity.ChatUser) (*Room, error) {
	s.roomsMu.Lock()
	defer s.roomsMu.Unlock()

	room := s.getOrCreateRoom(roomName)
	err := room.AddUser(user)
	if err != nil {
		return nil, err
	}

	return room, nil
}

func (s *ChatService) getOrCreateRoom(roomName string) *Room {
	if _, ok := s.rooms[roomName]; !ok {
		s.rooms[roomName] = NewRoom(roomName, s.producerFactory.Create())
		s.rooms[roomName].StartDeliveringMessages()

		log.Println("room", roomName, "created")
	}
	room := s.rooms[roomName]

	return room
}

func prepareMetaForClient(user entity.ChatUser) metadata.MD {
	encodedPublicKey := cryptotransfer.EncodePublicKeyToBase64(&user.ServerPrivateKey.PublicKey)

	return metadata.New(map[string]string{
		"uuid":       user.ID.String(),
		"cipher_key": encodedPublicKey,
	})
}

func (s *ChatService) disconnectUser(user entity.User, room *Room) {
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
		Type:     entity.UserDisconnected,
	})
}

func sendHandshake(msgStream chat.Chat_DoChattingServer) error {
	sendErr := msgStream.Send(&chat.MessageResponse{
		Content:  "handshake",
		FromName: "server",
		FromUuid: "",
	})

	return sendErr
}

func (s *ChatService) deleteRoomIfEmpty(room *Room) bool {
	s.roomsMu.Lock()
	defer s.roomsMu.Unlock()

	if ok := room.CloseIfEmpty(); ok {
		delete(s.rooms, room.Name)
		return true
	}

	return false
}

func (s *ChatService) handleInputMessages(user entity.ChatUser, room *Room) error {
	for {
		err := s.handleInputMessage(user, room)
		if errors.Is(err, ErrUserDisconnected) {
			return nil
		}

		if err != nil {
			log.Println("can't handle input message:", err)
			return status.Error(codes.Internal, "can't handle input message:"+err.Error())
		}
	}
}

func (s *ChatService) handleInputMessage(user entity.ChatUser, room *Room) error {
	in, receiveMessageErr := user.MessageStream.Recv()

	if receiveMessageErr == io.EOF {
		return ErrUserDisconnected
	}

	if st, ok := status.FromError(receiveMessageErr); ok && st.Code() == codes.Canceled {
		return ErrUserDisconnected
	}

	if receiveMessageErr != nil {
		return fmt.Errorf("can't receive message: %w", receiveMessageErr)
	}

	if sendMessageErr := decryptAndSendMessage(room, in.Content, user); sendMessageErr != nil {
		return fmt.Errorf("can't send message: %w", sendMessageErr)
	}

	return nil
}

func decryptAndSendMessage(room *Room, encryptedMessage string, user entity.ChatUser) error {
	decryptedMessage, decryptErr := cryptotransfer.DecryptRSAMessage(encryptedMessage, user.ServerPrivateKey)
	if decryptErr != nil {
		return fmt.Errorf("can't decrypt message: %w", decryptErr)
	}

	room.PushMessage(entity.Message{
		Content:  decryptedMessage,
		FromName: user.Name,
		FromUUID: user.ID,
		Type:     entity.UserMessage,
	})
	log.Printf("room %s: %s said %s", room.Name, user.Name, decryptedMessage)

	return nil
}
