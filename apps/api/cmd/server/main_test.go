package main

import (
	"api/internal/repository"
	"api/internal/service"
	"testing"
)

func TestOpenAPIContainsAllRoutes(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, api := newAPI(service.NewUserService(repository.NewPostgresUserRepository(nil)))

	paths := api.OpenAPI().Paths
	for _, path := range []string{"/health", "/api/v1/users/{id}"} {
		if paths[path] == nil {
			t.Errorf("OpenAPI document is missing %s", path)
		}
	}
}
