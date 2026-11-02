package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleUser    Role = "user"
	RoleEditor  Role = "editor"
	RoleChecker Role = "checker"
	RoleAdmin   Role = "admin"
)

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrDatabaseUnavailable = errors.New("database unavailable")
	ErrInvalidInput        = errors.New("invalid input")
)

type User struct {
	ID                        uuid.UUID  `json:"id" format:"uuid"`
	Email                     string     `json:"email" format:"email"`
	DisplayName               string     `json:"display_name"`
	OnboardingCompletedAt     *time.Time `json:"onboarding_completed_at,omitempty"`
	EmailNotificationsEnabled bool       `json:"email_notifications_enabled"`
	IsVerifiedEducator        bool       `json:"is_verified_educator"`
	Roles                     []Role     `json:"roles" enum:"user,editor,checker,admin"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
}

type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
}
