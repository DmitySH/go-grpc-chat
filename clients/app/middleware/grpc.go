package middleware

import (
	"context"
	"google.golang.org/grpc"
	"log"
)

func AuthInterceptor(ctx context.Context,
	desc *grpc.StreamDesc,
	cc *grpc.ClientConn,
	method string,
	streamer grpc.Streamer,
	opts ...grpc.CallOption) (grpc.ClientStream, error) {
	log.Println("auth logic")

	return streamer(ctx, desc, cc, method, opts...)
}
