package client

import (
	"context"
	"errors"
	"fmt"
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"io"
	"log"
)

var ErrUserStopChatting = errors.New("user stopped chatting")

type Config struct {
	ServerHost string
	ServerPort int
}

type ChatClient struct {
	config   Config
	metadata metadata.MD
}

func NewChatClient(config Config, username, room string) *ChatClient {
	return &ChatClient{
		config:   config,
		metadata: metadata.New(map[string]string{"username": username, "room": room}),
	}
}

func (c *ChatClient) DoChatting() error {
	conn, connErr := c.createGrpcConn()
	if connErr != nil {
		return fmt.Errorf("can't create grpc conn: %w", connErr)
	}
	defer conn.Close()

	grpcClient := chat.NewChatClient(conn)
	ctx := metadata.NewOutgoingContext(context.Background(), c.metadata)

	msgStream, startChattingErr := grpcClient.DoChatting(ctx)
	if startChattingErr != nil {
		return fmt.Errorf("failed connect to chat: %w", startChattingErr)
	}

	log.Println(c.metadata.Get("username")[0], "connected to room", c.metadata.Get("room")[0])

	defer func() {
		if closeSendErr := msgStream.CloseSend(); closeSendErr != nil {
			log.Println("can't close send: %w", closeSendErr)
		}
	}()

	readErrCh := make(chan error)
	writeErrCh := make(chan error)

	go c.readMessages(msgStream, readErrCh)
	go c.writeMessages(msgStream, writeErrCh)

	select {
	case err := <-writeErrCh:
		if !errors.Is(err, ErrUserStopChatting) {
			return fmt.Errorf("can't write to chat: %w", err)
		}
	case err := <-readErrCh:
		return fmt.Errorf("can't read from chat: %w", err)
	}

	return nil
}

func (c *ChatClient) createGrpcConn() (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conn, err := grpc.Dial(fmt.Sprintf("%s:%d",
		c.config.ServerHost,
		c.config.ServerPort),
		opts...,
	)

	if err != nil {
		return nil, fmt.Errorf("fail to dial %s:%d: %v", c.config.ServerHost,
			c.config.ServerPort, err)
	}

	return conn, nil
}

func (c *ChatClient) readMessages(inMsgStream chat.Chat_DoChattingClient, errCh chan<- error) {
	for {
		msg, err := inMsgStream.Recv()
		if err == io.EOF {
			close(errCh)
			return
		}

		if err != nil {
			errCh <- fmt.Errorf("can't read message: %w", err)
			close(errCh)
			return
		}

		fmt.Printf("%s: %s", msg.Username, msg.Content)
	}
}

func (c *ChatClient) writeMessages(outMsgStream chat.Chat_DoChattingClient, errCh chan<- error) {
	for {
		var msg string
		_, inputReadErr := fmt.Scanln(&msg)
		if inputReadErr != nil {
			return
		}
		if inputReadErr != nil {
			errCh <- fmt.Errorf("can't read user input: %w", inputReadErr)
		}

		if msg == "stop" {
			errCh <- ErrUserStopChatting
			close(errCh)
			return
		}

		req := &chat.MessageRequest{Content: msg}
		if err := outMsgStream.Send(req); err != nil {
			errCh <- fmt.Errorf("failed to send message: %w", err)
			close(errCh)
			return
		}
	}
}
