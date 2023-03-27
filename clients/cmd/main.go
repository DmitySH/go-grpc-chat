package main

import (
	"context"
	"fmt"
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"github.com/DmitySH/go-grpc-chat/pkg/config"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"log"
	"time"
)

const cfgPath = "configs/app.env"

type clientConfig struct {
	serverHost string
	serverPort int
}

func main() {
	config.MustLoadConfig(cfgPath)
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	clientCfg := clientConfig{
		serverHost: viper.GetString("SERVER_HOST"),
		serverPort: viper.GetInt("SERVER_PORT"),
	}

	conn, err := grpc.Dial(fmt.Sprintf("%s:%d",
		clientCfg.serverHost,
		clientCfg.serverPort),
		opts...,
	)

	if err != nil {
		log.Fatalf("fail to dial %s:%d: %v", clientCfg.serverHost,
			clientCfg.serverPort, err)
	}

	defer conn.Close()

	chatClient := chat.NewChatClient(conn)

	md := metadata.New(map[string]string{"username": "alex", "room": "hype"})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	msgStream, err := chatClient.DoChatting(ctx)
	if err != nil {
		log.Fatal("failed connect to chat:", err)
	}

	time.Sleep(time.Second)

	if closeSendErr := msgStream.CloseSend(); closeSendErr != nil {
		log.Fatal("can't close send:", closeSendErr)
	}

}
