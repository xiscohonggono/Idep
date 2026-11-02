package handler

import (
	"api/internal/domain"
	"api/internal/service"
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type GetUserInput struct {
	ID uuid.UUID `path:"id" format:"uuid" doc:"User UUID"`
}

type GetUserOutput struct {
	Body domain.User
}

func RegisterUserRoutes(api huma.API, svc *service.UserService) {
	huma.Register(api, huma.Operation{
		OperationID: "get-user",
		Method:      http.MethodGet,
		Path:        "/api/v1/users/{id}",
		Summary:     "Get user by ID",
		Tags:        []string{"users"},
		Errors:      []int{http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusServiceUnavailable},
	}, func(ctx context.Context, input *GetUserInput) (*GetUserOutput, error) {
		user, err := svc.GetUserProfile(ctx, input.ID)
		switch {
		case err == nil:
			return &GetUserOutput{Body: *user}, nil
		case errors.Is(err, domain.ErrInvalidInput):
			return nil, huma.Error422UnprocessableEntity("invalid user ID")
		case errors.Is(err, domain.ErrUserNotFound):
			return nil, huma.Error404NotFound("user not found")
		default:
			log.Printf("get-user failed: %v", err)
			return nil, huma.Error503ServiceUnavailable("database unavailable")
		}
	})
}
