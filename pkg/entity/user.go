package entity

import (
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	Name         string
	OutputStream chat.Chat_DoChattingServer
}
