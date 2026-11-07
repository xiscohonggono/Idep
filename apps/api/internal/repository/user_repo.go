package repository

import (
	"api/internal/domain"
	"api/internal/repository/db"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// queryTimeout is the OLTP query limit from guardrails §4.
const queryTimeout = 5 * time.Second

type PostgresUserRepository struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

func NewPostgresUserRepository(pool *pgxpool.Pool) *PostgresUserRepository {
	r := &PostgresUserRepository{}
	r.SetPool(pool)
	return r
}

// SetPool attaches the database pool after routes are registered, so the
// OpenAPI export never needs a database. Call it before serving requests.
func (r *PostgresUserRepository) SetPool(pool *pgxpool.Pool) {
	r.pool = pool
	if pool != nil {
		r.queries = db.New(pool)
	}
}

func (r *PostgresUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	if r.queries == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	userRow, err := r.queries.GetUserByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}

	return &domain.User{
		ID:    userRow.ID,
		Name:  userRow.Name,
		Email: userRow.Email,
	}, nil
}
