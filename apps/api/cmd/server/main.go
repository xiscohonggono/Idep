package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"api/internal/handler"
	"api/internal/repository"
	"api/internal/service"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type HealthOutput struct {
	Body struct {
		Status  string `json:"status" example:"ok"`
		Message string `json:"message" example:"Go API operational with pgxpool"`
	}
}

func main() {
	ctx := context.Background()

	// Initialize pgxpool connection
	dbURL := os.Getenv("DATABASE_URL")
	var pool *pgxpool.Pool

	if dbURL != "" {
		poolConfig, err := pgxpool.ParseConfig(dbURL)
		if err == nil {
			pool, _ = pgxpool.NewWithConfig(ctx, poolConfig)
		}
	}

	// 1. Wire sqlc repository
	userRepo := repository.NewPostgresUserRepository(pool)

	// 2. Wire business service layer
	userService := service.NewUserService(userRepo)

	// 3. Wire HTTP handler layer
	userHandler := handler.NewUserHandler(userService)

	// 4. Setup Chi router and Huma API adapter
	router := chi.NewMux()
	api := humachi.New(router, huma.DefaultConfig("Turbo Next Go API", "1.0.0"))

	// Register typed health check route with Huma
	huma.Register(api, huma.Operation{
		OperationID: "get-health",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Health check",
	}, func(ctx context.Context, input *struct{}) (*HealthOutput, error) {
		resp := &HealthOutput{}
		resp.Body.Status = "ok"
		resp.Body.Message = "Go API operational with pgxpool"
		return resp, nil
	})

	// Mount existing handlers under standard Chi routes
	router.Mount("/api/v1/users", userHandler)

	// Export OpenAPI contract flag
	if len(os.Args) > 1 && os.Args[1] == "--export-openapi" {
		b, err := json.MarshalIndent(api.OpenAPI(), "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal OpenAPI: %v", err)
		}
		if err := os.WriteFile("openapi.json", b, 0644); err != nil {
			log.Fatalf("Failed to write openapi.json: %v", err)
		}
		fmt.Println("Generated openapi.json successfully")
		return
	}

	port := ":8080"
	fmt.Printf("Go API server running on http://localhost%s (sqlc + pgx pool active)\n", port)
	fmt.Printf("OpenAPI interactive docs at http://localhost%s/docs\n", port)
	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}