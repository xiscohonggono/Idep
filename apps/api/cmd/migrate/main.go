// Command migrate applies or rolls back goose migrations in sql/schema.
// Run it from apps/api: go run ./cmd/migrate up|down
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "up" && os.Args[1] != "down") {
		fmt.Fprintln(os.Stderr, "usage: migrate up|down")
		os.Exit(2)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatalf("DATABASE_URL is not set")
	}

	// Driver errors can quote the URL, so they are not logged.
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("Invalid DATABASE_URL")
	}
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to reach the database at DATABASE_URL")
	}

	p, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("sql/schema"))
	if err != nil {
		log.Fatalf("Failed to load migrations: %v", err)
	}

	err = run(ctx, p, os.Args[1])
	p.Close()
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
}

func run(ctx context.Context, p *goose.Provider, command string) error {
	if command == "up" {
		results, err := p.Up(ctx)
		for _, result := range results {
			fmt.Println(result)
		}
		return err
	}

	result, err := p.Down(ctx)
	if errors.Is(err, goose.ErrNoNextVersion) {
		fmt.Println("nothing to roll back")
		return nil
	}
	if result != nil {
		fmt.Println(result)
	}
	return err
}
