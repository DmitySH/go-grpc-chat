package client

import (
	"context"
	"errors"
	"fmt"
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"io"
	"log"
)

var (
	ErrUserStopChatting   = errors.New("user stopped chatting")
	ErrServerDisconnected = errors.New("server disconnected")
)

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

	msgStream, startChatErr := grpcClient.DoChatting(ctx)
	if startChatErr != nil {
		return fmt.Errorf("failed connect to chat: %w", startChatErr)
	}

	log.Println(c.metadata.Get("username")[0], "connected to room", c.metadata.Get("room")[0])

	defer func() {
		if closeSendErr := msgStream.CloseSend(); closeSendErr != nil {
			log.Println("can't close send: ", closeSendErr)
		}
	}()

	return c.readAndWriteMessagesFromStream(msgStream)
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

func (c *ChatClient) readAndWriteMessagesFromStream(msgStream chat.Chat_DoChattingClient) error {
	readErrCh := make(chan error)
	writeErrCh := make(chan error)

	go c.readMessages(msgStream, readErrCh)
	go c.writeMessages(msgStream, writeErrCh)

	select {
	case err := <-readErrCh:
		return fmt.Errorf("can't read from chat: %w", err)
	case err := <-writeErrCh:
		if !errors.Is(err, ErrUserStopChatting) {
			return fmt.Errorf("can't write to chat: %w", err)
		}
	}

	return nil
}

func (c *ChatClient) readMessages(inMsgStream chat.Chat_DoChattingClient, errCh chan<- error) {
	defer close(errCh)

	for {
		msg, err := inMsgStream.Recv()
		if err == io.EOF {
			errCh <- ErrServerDisconnected
			return
		}

		if st, ok := status.FromError(err); ok && st.Code() != codes.OK {
			errCh <- err
			return
		}

		if err != nil {
			errCh <- fmt.Errorf("can't read message: %w", err)
			return
		}

		fmt.Printf("%s: %s", msg.Username, msg.Content)
	}
}

func (c *ChatClient) writeMessages(outMsgStream chat.Chat_DoChattingClient, errCh chan<- error) {
	defer close(errCh)

	for {
		var msg string
		_, inputReadErr := fmt.Scanln(&msg)

		if inputReadErr == io.EOF {
			errCh <- ErrUserStopChatting
			return
		}

		if inputReadErr != nil {
			errCh <- fmt.Errorf("can't read user input: %w", inputReadErr)
			return
		}

		req := &chat.MessageRequest{Content: msg}
		if err := outMsgStream.Send(req); err != nil {
			errCh <- fmt.Errorf("failed to send message: %w", err)
			return
		}
	}
}
