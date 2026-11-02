package main

import (
	"api/internal/llm"
	"api/internal/repository"
	"api/internal/service"
	"testing"
)

func TestOpenAPIContainsAllRoutes(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	analogyService := service.NewAnalogyService(repository.NewPostgresAnalogyRepository(nil), llm.NewOpenAIGenerator("", ""))
	_, api := newAPI(service.NewUserService(repository.NewPostgresUserRepository(nil)), analogyService)

	paths := api.OpenAPI().Paths
	for _, path := range []string{
		"/health",
		"/api/v1/users/{id}",
		"/api/v1/chapters/{chapter_id}/explanations",
		"/api/v1/highlights/{highlight_id}/explanations",
		"/api/v1/explanations/{explanation_id}/elaborate",
		"/api/v1/explanations/{explanation_id}/feedback",
	} {
		if paths[path] == nil {
			t.Errorf("OpenAPI document is missing %s", path)
		}
	}
}
