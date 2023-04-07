package main

import (
	"fmt"
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"github.com/DmitySH/go-grpc-chat/internal/services"
	"github.com/DmitySH/go-grpc-chat/pkg/config"
	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
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

	kafkaConfig := config.KafkaConfig{
		Addr:                   kafka.TCP(strings.Split(viper.GetString("KAFKA_HOSTS"), ",")...),
		Topic:                  viper.GetString("KAFKA_TOPIC"),
		AllowAutoTopicCreation: viper.GetBool("KAFKA_AUTO_TOPIC_CREATION"),
	}

	listener, err := net.Listen("tcp",
		fmt.Sprintf("%s:%d", serverCfg.host, serverCfg.port))
	if err != nil {
		log.Fatal("can't listen:", err)
	}
	defer listener.Close()

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	chatService := services.NewChatService(kafkaConfig)

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
