package main

import (
	"fmt"
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"github.com/DmitySH/go-grpc-chat/internal/services"
	"github.com/DmitySH/go-grpc-chat/pkg/config"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

const cfgPath = "configs/app.env"

type serverConfig struct {
	host string
	port int
}

func main() {
	config.LoadEnvConfig(cfgPath)
	serverCfg := serverConfig{
		host: viper.GetString("SERVER_HOST"),
		port: viper.GetInt("SERVER_PORT"),
	}

	listener, err := net.Listen("tcp",
		fmt.Sprintf("%s:%d", serverCfg.host, serverCfg.port))

	defer listener.Close()

	if err != nil {
		log.Fatal("can't listen:", err)
	}

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	chatService := services.NewChatService()

	chat.RegisterChatServer(grpcServer, chatService)

	log.Printf("starting server on %s:%d\n", serverCfg.host, serverCfg.port)

	stopChan := make(chan os.Signal, 2)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-stopChan
		log.Println("shutting down server")
		grpcServer.Stop()
	}()

	if serveErr := grpcServer.Serve(listener); serveErr != nil {
		log.Fatal("serving error:", err)
	}
	log.Println("server stopped")
}
