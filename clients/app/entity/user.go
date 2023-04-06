package entity

import (
	"crypto/rsa"
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"github.com/google/uuid"
)

type User struct {
	ID               uuid.UUID
	Name             string
	MessageStream    chat.Chat_DoChattingClient
	ServerPublicKey  *rsa.PublicKey
	ClientPrivateKey *rsa.PrivateKey
}
