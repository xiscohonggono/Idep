package domain

import (
	"context"
	"errors"
)

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrDatabaseUnavailable = errors.New("database unavailable")
)

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type UserRepository interface {
	GetByID(ctx context.Context, id string) (*User, error)
}
