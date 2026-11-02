package service

import (
	"api/internal/domain"
	"context"

	"github.com/google/uuid"
)

type UserService struct {
	repo domain.UserRepository
}

func NewUserService(repo domain.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetUserProfile(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if id == uuid.Nil {
		return nil, domain.ErrInvalidInput
	}
	return s.repo.GetByID(ctx, id)
}
