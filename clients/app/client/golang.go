package client

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"github.com/DmitySH/go-grpc-chat/api/chat"
	"github.com/DmitySH/go-grpc-chat/clients/app/entity"
	"github.com/DmitySH/go-grpc-chat/pkg/cryptotransfer"
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

const cryptoBits = 2048

var (
	ErrUserStopChatting   = errors.New("user stopped chatting")
	ErrServerDisconnected = errors.New("server disconnected")
	ErrUnsafeChat         = errors.New("can't use cipher for messages")
)

type Config struct {
	ServerHost string
	ServerPort int
}

type ChatClient struct {
	config   Config
	username string
	roomName string
}

func NewChatClient(config Config, username, room string) *ChatClient {
	return &ChatClient{
		config:   config,
		username: username,
		roomName: room,
	}
}

func (c *ChatClient) DoChatting() error {
	conn, connErr := c.createGrpcConn()
	if connErr != nil {
		return fmt.Errorf("can't create grpc conn: %w", connErr)
	}
	defer conn.Close()

	grpcClient := chat.NewChatClient(conn)
	clientKeyPair, generateKeyErr := cryptotransfer.GenerateKeyPair(cryptoBits)
	if generateKeyErr != nil {
		log.Println("can't generate key pair:", generateKeyErr)
		return ErrUnsafeChat
	}

	ctx := metadata.NewOutgoingContext(context.Background(), c.prepareMetaForServer(&clientKeyPair.PublicKey))

	msgStream, startChatErr := grpcClient.DoChatting(ctx)
	if startChatErr != nil {
		return fmt.Errorf("failed connect to chat: %w", startChatErr)
	}

	user, createUserErr := c.createChatUser(c.username, msgStream, clientKeyPair)
	if createUserErr != nil {
		return fmt.Errorf("can't create chat user: %w", createUserErr)
	}

	log.Println(c.username, "connected to room", c.roomName)

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

func (c *ChatClient) prepareMetaForServer(pubKey *rsa.PublicKey) metadata.MD {
	encodedPublicKey := cryptotransfer.EncodePublicKeyToBase64(pubKey)

	return metadata.New(map[string]string{"username": c.username, "room": c.roomName, "cipher_key": encodedPublicKey})
}

func (c *ChatClient) createChatUser(username string, msgStream chat.Chat_DoChattingClient,
	privateKey *rsa.PrivateKey) (entity.User, error) {
	md, getMdErr := msgStream.Header()
	if getMdErr != nil {
		return entity.User{}, fmt.Errorf("failed to get metadata: %w", getMdErr)
	}
	userUUID, getUUIDErr := getUUIDFromMetadata(md)
	if getUUIDErr != nil {
		return entity.User{}, fmt.Errorf("failed to get uuid from metadata: %w", getUUIDErr)
	}
	encodedServerPublicKey, getServerPublicKeyErr := getServerPublicKeyFromMetadata(md)
	if getServerPublicKeyErr != nil {
		return entity.User{}, fmt.Errorf("failed to get cipher key from metadata: %w", getServerPublicKeyErr)
	}

	serverPublicKey, decodeKeyErr := cryptotransfer.DecodePublicKeyFromBase64(encodedServerPublicKey)
	if decodeKeyErr != nil {
		return entity.User{}, fmt.Errorf("can't decode server's key from base64: %w", decodeKeyErr)
	}

	return entity.User{
		ID:               userUUID,
		Name:             username,
		MessageStream:    msgStream,
		ServerPublicKey:  serverPublicKey,
		ClientPrivateKey: privateKey,
	}, nil
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

func getServerPublicKeyFromMetadata(md metadata.MD) (string, error) {
	if len(md.Get("cipher_key")) == 0 {
		return "", errors.New("no cipher key in metadata")
	}

	return md.Get("cipher_key")[0], nil
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
			return
		}

		decryptedMessage, decryptErr := cryptotransfer.DecryptRSAMessage(msg.Content, user.ClientPrivateKey)
		if decryptErr != nil {
			errCh <- fmt.Errorf("can't decrypt message: %w", parseErr)
			return
		}

		if fromUserID != user.ID {
			fmt.Printf("%s: %s", msg.FromName, decryptedMessage)
		} else {
			if !strings.HasPrefix(msg.FromName, "room") {
				fmt.Printf("%s (you): %s", msg.FromName, decryptedMessage)
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

		cipherMessage, encryptErr := cryptotransfer.EncryptRSAMessage(msg, user.ServerPublicKey)
		if encryptErr != nil {
			errCh <- fmt.Errorf("can't encrypt user's message: %w", encryptErr)
			return
		}

		req := &chat.MessageRequest{Content: cipherMessage}
		if err := user.MessageStream.Send(req); err != nil {
			errCh <- fmt.Errorf("failed to send message: %w", err)
			return
		}
	}
}
