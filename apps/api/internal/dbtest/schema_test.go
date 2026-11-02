package dbtest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pressly/goose/v3"
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

func newChapter(t *testing.T, conn *pgx.Conn) uuid.UUID {
	t.Helper()

	id := uuid.New()
	mustExec(t, conn, `
		INSERT INTO chapters (id, subject_id, slug, title, display_order)
		SELECT $1, s.id, $2, 'Cells', 1 FROM subjects s WHERE s.slug = 'science'`, id, id.String())
	return id
}

func newVersion(t *testing.T, conn *pgx.Conn, chapter, author uuid.UUID, number int) uuid.UUID {
	t.Helper()

	id := uuid.New()
	mustExec(t, conn, `
		INSERT INTO chapter_versions (id, chapter_id, version_number, title, content_markdown, edit_summary, author_id)
		VALUES ($1, $2, $3, 'Cells', 'Cells make energy.', 'Seed', $4)`, id, chapter, number, author)
	return id
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

	t.Run("catalogs are seeded", func(t *testing.T) {
		for table, want := range map[string]int{"job_fields": 10, "subjects": 4} {
			var got int
			if err := conn.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("%s has %d rows, want %d", table, got, want)
			}
		}
	})

	t.Run("chapter versions are append-only with valid provenance", func(t *testing.T) {
		author := newUser(t, conn)
		chapter := newChapter(t, conn)
		version := newVersion(t, conn, chapter, author, 1)

		for _, statement := range []string{
			"UPDATE chapter_versions SET title = 'Tampered' WHERE id = $1",
			"DELETE FROM chapter_versions WHERE id = $1",
		} {
			pgErr := execError(t, conn, statement, version)
			if !strings.Contains(pgErr.Message, "Table chapter_versions is append-only") {
				t.Fatalf("%s: message = %q", statement, pgErr.Message)
			}
		}

		pgErr := execError(t, conn, `
			INSERT INTO chapter_versions (chapter_id, version_number, title, content_markdown, edit_summary, author_id, checker_id)
			VALUES ($1, 2, 'Cells', 'Text', 'Seed', $2, $2)`, chapter, author)
		wantConstraint(t, pgErr, "chk_version_provenance")
	})

	const insertHighlight = `
		INSERT INTO study_highlights (user_id, subject_id, chapter_id, chapter_version_id, selected_text, paragraph_count, start_offset, end_offset)
		SELECT $1, c.subject_id, c.id, $3, $4, $5, 0, 20 FROM chapters c WHERE c.id = $2`

	t.Run("highlights enforce selection bounds", func(t *testing.T) {
		user := newUser(t, conn)
		chapter := newChapter(t, conn)
		version := newVersion(t, conn, chapter, user, 1)

		pgErr := execError(t, conn, insertHighlight, user, chapter, version, "A long enough selection", 4)
		wantConstraint(t, pgErr, "study_highlights_paragraph_count_check")

		pgErr = execError(t, conn, insertHighlight, user, chapter, version, "  too short   ", 1)
		wantConstraint(t, pgErr, "study_highlights_selected_text_check")
	})

	t.Run("AI tables enforce their invariants", func(t *testing.T) {
		user := newUser(t, conn)
		chapter := newChapter(t, conn)
		version := newVersion(t, conn, chapter, user, 1)
		mustExec(t, conn, insertHighlight, user, chapter, version, "A long enough selection", 1)

		var highlight uuid.UUID
		if err := conn.QueryRow(context.Background(), "SELECT id FROM study_highlights WHERE user_id = $1", user).Scan(&highlight); err != nil {
			t.Fatal(err)
		}

		const insertExplanation = `
			INSERT INTO ai_explanations (id, highlight_id, user_id, job_field_code, is_everyday_fallback, analogy_steps_json, analogy_limits_json, model_identifier, prompt_version)
			VALUES ($1, $2, $3, $4, $5, '[]', $6, 'model', 'v1')`
		const limits = `[{"analogy_limit": "a", "accurate_explanation": "b"}]`

		pgErr := execError(t, conn, insertExplanation, uuid.New(), highlight, user, "fnb", false, "[]")
		wantConstraint(t, pgErr, "ai_explanations_analogy_limits_json_check")

		pgErr = execError(t, conn, insertExplanation, uuid.New(), highlight, user, "fnb", true, limits)
		wantConstraint(t, pgErr, "chk_work_field_or_fallback")

		explanation := uuid.New()
		mustExec(t, conn, insertExplanation, explanation, highlight, user, "fnb", false, limits)

		const insertElaboration = `
			INSERT INTO ai_elaborations (explanation_id, user_id, target_section, target_index, is_on_topic, elaboration_markdown, model_identifier)
			VALUES ($1, $2, $3, $4, $5, $6, 'model')`
		pgErr = execError(t, conn, insertElaboration, explanation, user, "step", nil, true, "More")
		wantConstraint(t, pgErr, "chk_elaboration_target_index")

		pgErr = execError(t, conn, insertElaboration, explanation, user, "general", nil, false, nil)
		wantConstraint(t, pgErr, "chk_elaboration_content_or_hint")

		const insertFeedback = `
			INSERT INTO ai_explanation_feedback (explanation_id, user_id, chapter_id, job_field_code, rating, comment)
			VALUES ($1, $2, $3, 'fnb', $4, $5)`
		pgErr = execError(t, conn, insertFeedback, explanation, user, chapter, "meh", nil)
		wantConstraint(t, pgErr, "ai_explanation_feedback_rating_check")

		pgErr = execError(t, conn, insertFeedback, explanation, user, chapter, "like", strings.Repeat("x", 501))
		wantConstraint(t, pgErr, "ai_explanation_feedback_comment_check")

		mustExec(t, conn, insertFeedback, explanation, user, chapter, "like", "Clicked")
		pgErr = execError(t, conn, insertFeedback, explanation, user, chapter, "dislike", nil)
		wantConstraint(t, pgErr, "ai_explanation_feedback_explanation_id_user_id_key")
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

	for {
		_, err := p.Down(ctx)
		if errors.Is(err, goose.ErrNoNextVersion) {
			break
		}
		if err != nil {
			t.Fatalf("down: %v", err)
		}
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
