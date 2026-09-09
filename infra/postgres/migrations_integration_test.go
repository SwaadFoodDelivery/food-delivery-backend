package postgres

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestMigrationsAndDemoSeed(t *testing.T) {
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MIGRATION_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	root := repositoryRoot(t)
	if err := RunMigrations(ctx, db, filepath.Join(root, "migrations")); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	seed, err := os.ReadFile(filepath.Join(root, "scripts", "seed_demo.sql"))
	if err != nil {
		t.Fatalf("read demo seed: %v", err)
	}
	if _, err := db.ExecContext(ctx, string(seed)); err != nil {
		t.Fatalf("apply demo seed: %v", err)
	}

	var version int
	var dirty bool
	if err := db.QueryRowContext(ctx, `SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&version, &dirty); err != nil {
		t.Fatalf("read migration state: %v", err)
	}
	if version != 27 || dirty {
		t.Fatalf("migration state version=%d dirty=%v, want version 27 and clean", version, dirty)
	}

	var restaurants int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM restaurants WHERE status = 'active' AND is_open = TRUE`).Scan(&restaurants); err != nil {
		t.Fatalf("read seeded restaurants: %v", err)
	}
	if restaurants < 5 {
		t.Fatalf("seeded restaurants=%d, want at least 5 active/open restaurants", restaurants)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve repository test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}
