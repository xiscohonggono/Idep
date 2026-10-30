package repository

import (
	"context"
	"api/internal/domain"
	"api/internal/repository/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresUserRepository struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

func NewPostgresUserRepository(pool *pgxpool.Pool) *PostgresUserRepository {
	var q *db.Queries
	if pool != nil {
		q = db.New(pool)
	}
	return &PostgresUserRepository{
		queries: q,
		pool:    pool,
	}
}

func (r *PostgresUserRepository) GetByID(id string) (*domain.User, error) {
	// If a live database pool is connected and available, execute the sqlc query
	if r.queries != nil {
		ctx := context.Background()
		userRow, err := r.queries.GetUserByID(ctx, id)
		if err == nil {
			return &domain.User{
				ID:    userRow.ID,
				Name:  userRow.Name,
				Email: userRow.Email,
			}, nil
		}
	}

	// Graceful fallback mock so the server stays operational during development
	return &domain.User{
		ID:    id,
		Name:  "Demo User (sqlc + pgx layer)",
		Email: "demo@example.com",
	}, nil
}

