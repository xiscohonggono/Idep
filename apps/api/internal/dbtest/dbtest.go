// Package dbtest gives each live database test its own migrated schema, because
// go test runs packages in parallel against a single DATABASE_URL.
package dbtest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// New creates and migrates a throwaway schema, dropped when the test ends, and
// returns a DSN scoped to it. It skips the test when DATABASE_URL is unset.
func New(t *testing.T) string {
	t.Helper()

	baseDSN := os.Getenv("DATABASE_URL")
	if baseDSN == "" {
		t.Skip("DATABASE_URL not set; live database test skipped")
	}

	schemaDSN, schema := scopedDSN(t, baseDSN)
	exec(t, baseDSN, "CREATE SCHEMA "+schema)
	t.Cleanup(func() { exec(t, baseDSN, "DROP SCHEMA "+schema+" CASCADE") })

	p := newProvider(t, schemaDSN)
	if _, err := p.Up(context.Background()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return schemaDSN
}

// scopedDSN returns baseDSN with search_path set to a new random schema name.
func scopedDSN(t *testing.T, baseDSN string) (dsn, schema string) {
	t.Helper()

	u, err := url.Parse(baseDSN)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		// The parse error can quote the URL, so it is not logged.
		t.Fatal("DATABASE_URL must be a postgres:// URL")
	}

	suffix := make([]byte, 6)
	rand.Read(suffix)
	schema = "t_" + hex.EncodeToString(suffix)

	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	return u.String(), schema
}

func exec(t *testing.T, dsn, statement string) {
	t.Helper()

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("connect to DATABASE_URL failed")
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, statement); err != nil {
		t.Fatalf("%s: %v", statement, err)
	}
}

// newProvider returns a goose provider for the schema migrations, closed when
// the test ends. The path is resolved from this file, not the working directory.
func newProvider(t *testing.T, dsn string) *goose.Provider {
	t.Helper()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal("open DATABASE_URL failed")
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate dbtest source file")
	}
	migrations := filepath.Join(filepath.Dir(thisFile), "..", "..", "sql", "schema")

	p, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS(migrations))
	if err != nil {
		db.Close()
		t.Fatalf("create goose provider: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}
