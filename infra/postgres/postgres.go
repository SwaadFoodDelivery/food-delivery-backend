package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"food-delivery-backend/pkg/config"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func NewPostgresDB(ctx context.Context, cfg *config.Config) (*sqlx.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.User, cfg.Postgres.Password, cfg.Postgres.DBName, cfg.Postgres.SSLMode)
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(cfg.Postgres.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Postgres.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.Postgres.ConnMaxLifetimeMin) * time.Minute)

	if err := pingWithRetry(ctx, db.DB, 10, 2*time.Second); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := ensureExtensions(ctx, db.DB); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func pingWithRetry(ctx context.Context, db *sql.DB, attempts int, interval time.Duration) error {
	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := db.PingContext(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	return lastErr
}

func ensureExtensions(ctx context.Context, db *sql.DB) error {
	queries := []string{
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`,
		`CREATE EXTENSION IF NOT EXISTS "postgis"`,
		`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,
	}
	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}
