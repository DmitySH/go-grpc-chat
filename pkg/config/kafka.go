package config

import "net"

type KafkaConfig struct {
	Addr                   net.Addr
	Topic                  string
	AllowAutoTopicCreation bool
}
