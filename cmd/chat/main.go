package main

import (
	"github.com/DmitySH/go-grpc-chat/internal/chat"
	"github.com/DmitySH/go-grpc-chat/pkg/config"
)

const cfgPath = "configs/app.env"

func main() {
	config.LoadEnvConfig(cfgPath)
	chat.Run()
}
