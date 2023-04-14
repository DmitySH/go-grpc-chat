package chat

import (
	"fmt"
	"github.com/DmitySH/go-grpc-chat/internal/factory"
	"github.com/DmitySH/go-grpc-chat/internal/middleware"
	"github.com/DmitySH/go-grpc-chat/internal/repository"
	"github.com/DmitySH/go-grpc-chat/internal/services"
	"github.com/DmitySH/go-grpc-chat/pkg/api/chat"
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

type serverConfig struct {
	host string
	port int
}

func Run() {
	serverCfg := serverConfig{
		host: viper.GetString("SERVER_HOST"),
		port: viper.GetInt("SERVER_PORT"),
	}

	producerFactory := factory.NewKafkaFactory(
		kafka.TCP(strings.Split(viper.GetString("KAFKA_HOSTS"), ",")...),
		viper.GetString("KAFKA_TOPIC"),
		viper.GetBool("KAFKA_AUTO_TOPIC_CREATION"))

	db, connectDbErr := repository.NewPostgresDB(repository.Config{
		Host:     viper.GetString("DB_HOST"),
		Port:     viper.GetString("DB_PORT"),
		Username: viper.GetString("DB_USERNAME"),
		Password: viper.GetString("DB_PASSWORD"),
		DBName:   viper.GetString("DB_NAME"),
		SSLMode:  viper.GetString("DB_SSL_MODE"),
	})
	if connectDbErr != nil {
		log.Fatal("can't initialize db instance:", connectDbErr)
	}

	userRepo := repository.NewUserRepository(db)
	_ = userRepo

	chatService := services.NewChatService(producerFactory)

	var opts = []grpc.ServerOption{
		grpc.StreamInterceptor(middleware.AuthInterceptor),
	}
	grpcServer := grpc.NewServer(opts...)

	chat.RegisterChatServer(grpcServer, chatService)

	runAndShutdownServer(serverCfg, grpcServer)
}

func runAndShutdownServer(serverCfg serverConfig, grpcServer *grpc.Server) {
	listener, err := net.Listen("tcp",
		fmt.Sprintf("%s:%d", serverCfg.host, serverCfg.port))
	if err != nil {
		log.Fatal("can't listen:", err)
	}
	defer listener.Close()

	log.Printf("starting server on %s:%d\n", serverCfg.host, serverCfg.port)

	stopChan := make(chan os.Signal, 2)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-stopChan
		log.Println("shutting down server")
		grpcServer.Stop()
	}()

	if serveErr := grpcServer.Serve(listener); serveErr != nil {
		log.Fatal("serving error:", serveErr)
	}
	log.Println("server stopped")
}
