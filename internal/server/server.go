package server

import (
	"fmt"
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"github.com/DmitySH/go-grpc-chat/internal/chatroom"
	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	"io"
	"log"
)

type ChatServer struct {
	chat.UnimplementedChatServer
	rooms map[string]*chatroom.Room
}

func NewChatServer() *ChatServer {
	return &ChatServer{
		rooms: make(map[string]*chatroom.Room),
	}
}

func (s *ChatServer) DoChatting(msgStream chat.Chat_DoChattingServer) error {
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

	if _, ok = s.rooms[roomName]; !ok {
		s.rooms[roomName] = chatroom.NewRoom()
		s.rooms[roomName].StartDeliveringMessages()
	}
	room := s.rooms[roomName]

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
			return err
		}

		room.PushMessage(chatroom.Message{
			Content: in.Content,
			From:    username,
		})
		log.Printf("room %s: %s said %s", roomName, username, in.Content)
	}
}

func usernameAndRoomInMetadata(md metadata.MD) error {
	if len(md.Get("username")) == 0 {
		return fmt.Errorf("no username in metadata")
	}
	if len(md.Get("username")) == 0 {
		return fmt.Errorf("no username in metadata")
	}

	return nil
}
