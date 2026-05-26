package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	mp "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(ctx context.Context, db *sql.DB, migrationsPath string) error {
	driver, err := mp.WithInstance(db, &mp.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithDatabaseInstance(fmt.Sprintf("file://%s", migrationsPath), "postgres", driver)
	if err != nil {
		return err
	}
	errCh := make(chan error, 1)
	go func() {
		if runErr := m.Up(); runErr != nil && runErr != migrate.ErrNoChange {
			errCh <- runErr
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case runErr := <-errCh:
		return runErr
	}
}
