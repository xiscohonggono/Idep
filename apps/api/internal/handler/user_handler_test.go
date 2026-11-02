package handler

import (
	"api/internal/domain"
	"api/internal/repository"
	"api/internal/service"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/uuid"
)

const userPath = "/api/v1/users/00000000-0000-0000-0000-000000000101"

type fakeUserRepo struct {
	user *domain.User
	err  error
}

func (f fakeUserRepo) GetByID(context.Context, uuid.UUID) (*domain.User, error) {
	return f.user, f.err
}

func TestGetUser(t *testing.T) {
	createdAt := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	found := &domain.User{
		ID:          uuid.MustParse("00000000-0000-0000-0000-000000000101"),
		Email:       "a@example.com",
		DisplayName: "Ada",
		Roles:       []domain.Role{domain.RoleUser},
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}

	tests := []struct {
		name   string
		repo   domain.UserRepository
		path   string
		status int
	}{
		{"nil pool repository is unavailable", repository.NewPostgresUserRepository(nil), userPath, http.StatusServiceUnavailable},
		{"unavailable sentinel", fakeUserRepo{err: domain.ErrDatabaseUnavailable}, userPath, http.StatusServiceUnavailable},
		{"other repository error", fakeUserRepo{err: errors.New("connection reset")}, userPath, http.StatusServiceUnavailable},
		{"not found", fakeUserRepo{err: domain.ErrUserNotFound}, userPath, http.StatusNotFound},
		{"malformed uuid", fakeUserRepo{user: found}, "/api/v1/users/not-a-uuid", http.StatusUnprocessableEntity},
		{"nil uuid", fakeUserRepo{user: found}, "/api/v1/users/00000000-0000-0000-0000-000000000000", http.StatusUnprocessableEntity},
		{"found", fakeUserRepo{user: found}, userPath, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, api := humatest.New(t)
			RegisterUserRoutes(api, service.NewUserService(tt.repo))

			resp := api.Get(tt.path)
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
			if !reflect.DeepEqual(got, *found) {
				t.Fatalf("body = %+v, want %+v", got, *found)
			}
		})
	}
}
