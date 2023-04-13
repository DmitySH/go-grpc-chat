package entity

import (
	"crypto/rsa"
	"github.com/DmitySH/go-grpc-chat/pkg/api/chat"
	"github.com/google/uuid"
)

type User struct {
	ID               uuid.UUID
	Name             string
	MessageStream    chat.Chat_DoChattingServer
	ServerPrivateKey *rsa.PrivateKey
	ClientPublicKey  *rsa.PublicKey
}
