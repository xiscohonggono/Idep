package service

import (
	"api/internal/domain"
	"errors"
)

type UserService struct {
	repo domain.UserRepository
}

func NewUserService(repo domain.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetUserProfile(id string) (*domain.User, error) {
	if id == "" {
		return nil, errors.New("user ID cannot be empty")
	}
	return s.repo.GetByID(id)
}