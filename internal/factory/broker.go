package factory

import (
	"github.com/DmitySH/go-grpc-chat/internal/broker"
	"github.com/DmitySH/go-grpc-chat/internal/services"
	"net"
)

type KafkaFactory struct {
	addr                   net.Addr
	topic                  string
	allowAutoTopicCreation bool
}

func NewKafkaFactory(addr net.Addr, topic string, allowAutoTopicCreation bool) *KafkaFactory {
	return &KafkaFactory{
		addr:                   addr,
		topic:                  topic,
		allowAutoTopicCreation: allowAutoTopicCreation,
	}
}

func (k *KafkaFactory) Create() services.Producer {
	return broker.NewKafkaBroker(k.addr, k.topic, k.allowAutoTopicCreation)
}
