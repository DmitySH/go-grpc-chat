package chatroom

import "github.com/DmitySH/go-grpc-chat/api/chat"

type User struct {
	Name         string
	OutputStream chat.Chat_DoChattingServer
}
