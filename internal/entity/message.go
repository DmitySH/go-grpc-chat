package entity

import "github.com/google/uuid"

type MessageType int

const (
	UserConnected MessageType = iota
	UserMessage
	UserDisconnected
)

type Message struct {
	Content  string
	FromName string
	FromUUID uuid.UUID
	Type     MessageType
}
