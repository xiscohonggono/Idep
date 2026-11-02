package dbtest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func connect(t *testing.T, dsn string) *pgx.Conn {
	t.Helper()

	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal("connect to test schema failed")
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	return conn
}

func mustExec(t *testing.T, conn *pgx.Conn, statement string, args ...any) {
	t.Helper()

	if _, err := conn.Exec(context.Background(), statement, args...); err != nil {
		t.Fatalf("%s: %v", statement, err)
	}
}

func execError(t *testing.T, conn *pgx.Conn, statement string, args ...any) *pgconn.PgError {
	t.Helper()

	_, err := conn.Exec(context.Background(), statement, args...)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("%s: got error %v, want a PostgreSQL error", statement, err)
	}
	return pgErr
}

func wantConstraint(t *testing.T, pgErr *pgconn.PgError, constraint string) {
	t.Helper()

	if pgErr.ConstraintName != constraint {
		t.Fatalf("violated constraint = %q, want %q (%s)", pgErr.ConstraintName, constraint, pgErr.Message)
	}
}

func newUser(t *testing.T, conn *pgx.Conn) uuid.UUID {
	t.Helper()

	id := uuid.New()
	mustExec(t, conn, "INSERT INTO users (id, email, display_name) VALUES ($1, $2, 'Ada')", id, id.String()+"@example.com")
	return id
}

// schemaObjects returns how many of the Stage 0 tables and functions exist.
func schemaObjects(t *testing.T, conn *pgx.Conn) (tables, functions int) {
	t.Helper()

	ctx := context.Background()
	if err := conn.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_name IN ('users', 'user_roles', 'audit_events')`,
	).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.routines
		WHERE routine_schema = current_schema() AND routine_name = 'prevent_immutable_table_mutation'`,
	).Scan(&functions); err != nil {
		t.Fatal(err)
	}
	return tables, functions
}

func TestSchemaInvariants(t *testing.T) {
	conn := connect(t, New(t))

	const grantRole = "INSERT INTO user_roles (user_id, role) VALUES ($1, $2)"

	t.Run("tables exist", func(t *testing.T) {
		if tables, _ := schemaObjects(t, conn); tables != 3 {
			t.Fatalf("found %d of 3 tables", tables)
		}
	})

	t.Run("editor grant requires accepted guidelines", func(t *testing.T) {
		pgErr := execError(t, conn, grantRole, newUser(t, conn), "editor")
		wantConstraint(t, pgErr, "chk_editor_guidelines")
	})

	t.Run("revocation requires a non-blank reason", func(t *testing.T) {
		user := newUser(t, conn)
		mustExec(t, conn, grantRole, user, "checker")

		pgErr := execError(t, conn,
			"UPDATE user_roles SET revoked_at = NOW(), revocation_reason = '   ' WHERE user_id = $1", user)
		wantConstraint(t, pgErr, "chk_revocation_reason")
	})

	t.Run("one active grant per user and role", func(t *testing.T) {
		user := newUser(t, conn)
		mustExec(t, conn, grantRole, user, "checker")

		wantConstraint(t, execError(t, conn, grantRole, user, "checker"), "idx_user_roles_active_unique")

		mustExec(t, conn,
			"UPDATE user_roles SET revoked_at = NOW(), revocation_reason = 'rotation' WHERE user_id = $1", user)
		mustExec(t, conn, grantRole, user, "checker")
	})

	t.Run("audit events are append-only", func(t *testing.T) {
		id := uuid.New()
		mustExec(t, conn, `
			INSERT INTO audit_events (id, event_type, target_type, target_id, request_id)
			VALUES ($1, 'role.granted', 'user', $1, 'req-1')`, id)

		for _, statement := range []string{
			"UPDATE audit_events SET event_type = 'tampered' WHERE id = $1",
			"DELETE FROM audit_events WHERE id = $1",
		} {
			pgErr := execError(t, conn, statement, id)
			if !strings.Contains(pgErr.Message, "Table audit_events is append-only") {
				t.Fatalf("%s: message = %q", statement, pgErr.Message)
			}
		}
	})

	t.Run("display name cannot be blank", func(t *testing.T) {
		id := uuid.New()
		pgErr := execError(t, conn,
			"INSERT INTO users (id, email, display_name) VALUES ($1, $2, '  ')", id, id.String()+"@example.com")
		wantConstraint(t, pgErr, "users_display_name_check")
	})
}

func TestMigrationRollback(t *testing.T) {
	dsn := New(t)
	conn := connect(t, dsn)
	p := newProvider(t, dsn)
	ctx := context.Background()

	if _, err := p.Down(ctx); err != nil {
		t.Fatalf("down: %v", err)
	}
	if tables, functions := schemaObjects(t, conn); tables != 0 || functions != 0 {
		t.Fatalf("after down: %d tables, %d functions remain", tables, functions)
	}

	if _, err := p.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}
	if tables, functions := schemaObjects(t, conn); tables != 3 || functions != 1 {
		t.Fatalf("after up: %d tables, %d functions, want 3 and 1", tables, functions)
	}
}
