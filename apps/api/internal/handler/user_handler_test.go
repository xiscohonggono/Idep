package handler

import (
	"api/internal/domain"
	"api/internal/repository"
	"api/internal/service"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
)

type fakeUserRepo struct {
	user *domain.User
	err  error
}

func (f fakeUserRepo) GetByID(context.Context, string) (*domain.User, error) {
	return f.user, f.err
}

func TestGetUser(t *testing.T) {
	found := &domain.User{ID: "101", Email: "a@example.com", Name: "Ada"}

	tests := []struct {
		name   string
		repo   domain.UserRepository
		status int
	}{
		{"nil pool repository is unavailable", repository.NewPostgresUserRepository(nil), http.StatusServiceUnavailable},
		{"unavailable sentinel", fakeUserRepo{err: domain.ErrDatabaseUnavailable}, http.StatusServiceUnavailable},
		{"other repository error", fakeUserRepo{err: errors.New("connection reset")}, http.StatusServiceUnavailable},
		{"not found", fakeUserRepo{err: domain.ErrUserNotFound}, http.StatusNotFound},
		{"found", fakeUserRepo{user: found}, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, api := humatest.New(t)
			RegisterUserRoutes(api, service.NewUserService(tt.repo))

			resp := api.Get("/api/v1/users/101")
			if resp.Code != tt.status {
				t.Fatalf("status = %d, want %d: %s", resp.Code, tt.status, resp.Body)
			}
			if tt.status != http.StatusOK {
				return
			}

			var got domain.User
			if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got != *found {
				t.Fatalf("body = %+v, want %+v", got, *found)
			}
		})
	}
}
