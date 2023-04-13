package entity

import (
	"github.com/google/uuid"
	"time"
)

type BrokerLoggingMessage struct {
	MessageUUID    uuid.UUID
	Timestamp      time.Time
	MessageContent string
	FromUserUUID   uuid.UUID
	ToUserUUID     uuid.UUID
}
