package server

import (
	"fmt"
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"google.golang.org/grpc/metadata"
	"log"
)

type ChatServer struct {
	chat.UnimplementedChatServer
}

func NewChatServer() *ChatServer {
	return &ChatServer{}
}

func (s *ChatServer) DoChatting(inMsgStream chat.Chat_DoChattingServer) error {
	md, ok := metadata.FromIncomingContext(inMsgStream.Context())
	if !ok {
		return fmt.Errorf("no metadata in request")
	}

	if err := usernameAndRoomInMetadata(md); err != nil {
		return err
	}

	username := md.Get("username")[0]
	room := md.Get("room")[0]

	log.Println("user", username, "connected to room", room)

	return nil
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
