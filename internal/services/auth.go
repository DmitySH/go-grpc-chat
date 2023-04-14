package services

import (
	"context"
	"github.com/DmitySH/go-grpc-chat/internal/entity"
)

type AuthRepository interface {
	CreateUser(ctx context.Context, user entity.User) error
}

type AuthService struct {
	repo AuthRepository
}

func NewAuthService(repo AuthRepository) *AuthService {
	return &AuthService{
		repo: repo,
	}
}
