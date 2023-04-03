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
	config.MustLoadEnvConfig(cfgPath)

	clientCfg := client.Config{
		ServerHost: viper.GetString("SERVER_HOST"),
		ServerPort: viper.GetInt("SERVER_PORT"),
	}

	username, room := mustReadUser()
	chatClient := client.NewChatClient(clientCfg, username, room)

	err := chatClient.DoChatting()
	if err != nil {
		log.Fatal(err)
	}
}

func mustReadUser() (string, string) {
	var username, room string
	fmt.Println("Enter your name:")
	_, err := fmt.Scanln(&username)
	if err != nil {
		log.Fatal("can't read username", err)
	}

	fmt.Println("Enter room:")
	_, err = fmt.Scanln(&room)
	if err != nil {
		log.Fatal("can't read room", err)
	}

	return username, room
}
