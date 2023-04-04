package client

import (
	"context"
	"errors"
	"fmt"
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"github.com/DmitySH/go-grpc-chat/clients/entity"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"io"
	"log"
	"strings"
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

	md, getMdErr := msgStream.Header()
	if getMdErr != nil {
		return fmt.Errorf("failed to get metadata: %w", getMdErr)
	}
	userUUID, getUUIDErr := getUUIDFromMetadata(md)
	if getUUIDErr != nil {
		return fmt.Errorf("failed to get uuid from metadata: %w", getUUIDErr)
	}

	user := entity.User{
		ID:            userUUID,
		Name:          c.metadata.Get("username")[0],
		MessageStream: msgStream,
	}

	log.Println(c.metadata.Get("username")[0], "connected to room", c.metadata.Get("room")[0])

	defer func() {
		if closeSendErr := msgStream.CloseSend(); closeSendErr != nil {
			log.Println("can't close send: ", closeSendErr)
		}
	}()

	return c.readAndWriteMessagesFromStream(user)
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

func getUUIDFromMetadata(md metadata.MD) (uuid.UUID, error) {
	if len(md.Get("uuid")) == 0 {
		return uuid.UUID{}, errors.New("no uuid in metadata")
	}
	userUUID, parseErr := uuid.Parse(md.Get("uuid")[0])
	if parseErr != nil {
		return uuid.UUID{}, fmt.Errorf("can't parse uuid from metadata: %w", parseErr)
	}

	return userUUID, nil
}

func (c *ChatClient) readAndWriteMessagesFromStream(user entity.User) error {
	readErrCh := make(chan error)
	writeErrCh := make(chan error)

	go c.readMessages(user, readErrCh)
	go c.writeMessages(user, writeErrCh)

	select {
	case err := <-readErrCh:
		if errors.Is(err, ErrServerDisconnected) {
			log.Println(err)
			return nil
		}
		return fmt.Errorf("can't read from chat: %w", err)
	case err := <-writeErrCh:
		if errors.Is(err, ErrUserStopChatting) {
			return nil
		}
		return fmt.Errorf("can't write to chat: %w", err)
	}
}

func (c *ChatClient) readMessages(user entity.User, errCh chan<- error) {
	defer close(errCh)

	for {
		msg, err := user.MessageStream.Recv()
		if err == io.EOF {
			errCh <- ErrServerDisconnected
			return
		}
		if status.Convert(err).Code() == codes.Unavailable {
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

		fromUserID, parseErr := uuid.Parse(msg.FromUuid)
		if parseErr != nil {
			errCh <- fmt.Errorf("can't parse uuid: %w", parseErr)
		}
		if fromUserID != user.ID {
			fmt.Printf("%s: %s", msg.Username, msg.Content)
		} else {
			if !strings.HasPrefix(msg.Username, "room") {
				fmt.Printf("%s (you): %s", msg.Username, msg.Content)
			}
		}
	}
}

func (c *ChatClient) writeMessages(user entity.User, errCh chan<- error) {
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
		if err := user.MessageStream.Send(req); err != nil {
			errCh <- fmt.Errorf("failed to send message: %w", err)
			return
		}
	}
}
