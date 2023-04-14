package middleware

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func AuthInterceptor(srv interface{},
	ss grpc.ServerStream,
	_ *grpc.StreamServerInfo,
	handler grpc.StreamHandler) error {
	ctx := ss.Context()

	checkMdErr := checkMetadata(ctx)
	if checkMdErr != nil {
		return status.Error(codes.PermissionDenied, fmt.Sprintf("incorrect metadata: %v", checkMdErr))
	}

	return handler(srv, ss)
}

func checkMetadata(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errors.New("no metadata")
	}
	if len(md.Get("username")) == 0 {
		return errors.New("no username in metadata")
	}
	if len(md.Get("room")) == 0 {
		return errors.New("no room in metadata")
	}
	if len(md.Get("cipher_key")) == 0 {
		return errors.New("no cipher key in metadata")
	}

	return nil
}
