package repository

import (
	"api/internal/dbtest"
	"api/internal/domain"
	"api/internal/repository/db"
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestGetByIDNilPool(t *testing.T) {
	_, err := NewPostgresUserRepository(nil).GetByID(context.Background(), uuid.New())
	if !errors.Is(err, domain.ErrDatabaseUnavailable) {
		t.Fatalf("err = %v, want ErrDatabaseUnavailable", err)
	}
}

func wantRoles(t *testing.T, repo *PostgresUserRepository, id uuid.UUID, want ...domain.Role) {
	t.Helper()

	user, err := repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(user.Roles, want) {
		t.Fatalf("roles = %v, want %v", user.Roles, want)
	}
}

func TestGetByIDLive(t *testing.T) {
	dsn := dbtest.New(t)
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("open test pool failed")
	}
	t.Cleanup(pool.Close)

	repo := NewPostgresUserRepository(pool)
	queries := db.New(pool)
	id := uuid.New()

	// NOW() is fixed per transaction, so grants in one transaction tie on
	// granted_at. The editor grant commits separately to pin the grant order.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	txQueries := queries.WithTx(tx)
	if _, err := txQueries.CreateUser(ctx, db.CreateUserParams{ID: id, Email: "ada@example.com", DisplayName: "Ada"}); err != nil {
		t.Fatal(err)
	}
	if _, err := txQueries.GrantUserRole(ctx, db.GrantUserRoleParams{UserID: id, Role: "user"}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	txQueries = queries.WithTx(tx)
	accepted := time.Now()
	if _, err := txQueries.GrantUserRole(ctx, db.GrantUserRoleParams{UserID: id, Role: "editor", GuidelinesAcceptedAt: &accepted}); err != nil {
		t.Fatal(err)
	}
	if _, err := txQueries.InsertAuditEvent(ctx, db.InsertAuditEventParams{
		EventType:    "role.granted",
		TargetType:   "user",
		TargetID:     id,
		MetadataJson: []byte("{}"),
		RequestID:    "req-1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	user, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != id || user.Email != "ada@example.com" || user.DisplayName != "Ada" {
		t.Fatalf("user = %+v", user)
	}
	wantRoles(t, repo, id, domain.RoleUser, domain.RoleEditor)

	reason := "guidelines breach"
	if _, err := queries.RevokeUserRole(ctx, db.RevokeUserRoleParams{UserID: id, Role: "editor", RevocationReason: &reason}); err != nil {
		t.Fatal(err)
	}
	wantRoles(t, repo, id, domain.RoleUser)

	if _, err := repo.GetByID(ctx, uuid.New()); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("unknown user: err = %v, want ErrUserNotFound", err)
	}

	if _, err := pool.Exec(ctx, "UPDATE users SET deleted_at = NOW() WHERE id = $1", id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(ctx, id); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("deleted user: err = %v, want ErrUserNotFound", err)
	}
}
