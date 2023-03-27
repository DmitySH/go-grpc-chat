package main

import (
	"fmt"
	"github.com/DmitySH/go-grpc-chat/clients/client"
	"github.com/DmitySH/go-grpc-chat/pkg/config"
	"github.com/spf13/viper"
	"log"
)

const cfgPath = "configs/app.env"

func main() {
	config.MustLoadConfig(cfgPath)

	clientCfg := client.Config{
		ServerHost: viper.GetString("SERVER_HOST"),
		ServerPort: viper.GetInt("SERVER_PORT"),
	}

	var username, room string
	fmt.Scanln(&username)
	fmt.Scanln(&room)
	chatClient := client.NewChatClient(clientCfg, username, room)

	err := chatClient.DoChatting()
	if err != nil {
		log.Fatal(err)
	}
}
