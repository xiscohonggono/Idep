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
	// Routes are registered before any database work so that exporting the
	// OpenAPI contract needs neither DATABASE_URL nor a reachable database.
	userRepo := repository.NewPostgresUserRepository(nil)
	router, api := newAPI(service.NewUserService(userRepo))

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

	userRepo.SetPool(openPool(context.Background()))

	port := ":8080"
	fmt.Printf("Go API server running on http://localhost%s\n", port)
	fmt.Printf("OpenAPI interactive docs at http://localhost%s/docs\n", port)
	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// newAPI builds the router and registers every Huma operation.
func newAPI(userService *service.UserService) (*chi.Mux, huma.API) {
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

	handler.RegisterUserRoutes(api, userService)

	return router, api
}

// openPool returns nil when DATABASE_URL is unset, so DB-backed operations
// answer 503. A configured but invalid database is fatal.
func openPool(ctx context.Context) *pgxpool.Pool {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Println("WARNING: DATABASE_URL is not set; database-backed operations will return 503")
		return nil
	}

	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		// The parse error can quote the URL, so it is not logged.
		log.Fatalf("Invalid DATABASE_URL")
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("Failed to create database pool: %v", err)
	}
	return pool
}
