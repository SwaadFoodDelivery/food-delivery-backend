package postgres

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
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
	if version != 28 || dirty {
		t.Fatalf("migration state version=%d dirty=%v, want version 28 and clean", version, dirty)
	}

	var restaurants int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM restaurants WHERE status = 'active' AND is_open = TRUE`).Scan(&restaurants); err != nil {
		t.Fatalf("read seeded restaurants: %v", err)
	}
	if restaurants < 5 {
		t.Fatalf("seeded restaurants=%d, want at least 5 active/open restaurants", restaurants)
	}
}

func TestOnboardingApprovalMigration28(t *testing.T) {
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MIGRATION_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := RunMigrations(ctx, db, filepath.Join(repositoryRoot(t), "migrations")); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile(filepath.Join(repositoryRoot(t), "migrations/000028_reconcile_onboarding_approval.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	// Roll back every fixture and corrective update, even if an assertion fails.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	ids := map[string]string{}
	for _, status := range []string{"draft", "pending_verification", "rejected", "approved", "seeded"} {
		id := uuid.NewString()
		ids[status] = id
		if _, err := tx.ExecContext(ctx, `INSERT INTO users (user_id, phone, name, role, onboarding_complete) VALUES ($1, $2, 'Migration test', 'driver', TRUE)`, id, id[:14]); err != nil {
			t.Fatal(err)
		}
		if status != "seeded" {
			if _, err := tx.ExecContext(ctx, `INSERT INTO onboardings (user_id, role, status) VALUES ($1, 'driver', $2)`, id, status); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := tx.ExecContext(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
		for status, id := range ids {
			var complete bool
			if err := tx.QueryRowContext(ctx, `SELECT onboarding_complete FROM users WHERE user_id=$1`, id).Scan(&complete); err != nil {
				t.Fatal(err)
			}
			want := status == "approved" || status == "seeded"
			if complete != want {
				t.Errorf("pass %d status %s complete=%v want=%v", i, status, complete, want)
			}
		}
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
