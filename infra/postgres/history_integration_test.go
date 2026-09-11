package postgres

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestStatusHistoryMigration29(t *testing.T) {
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MIGRATION_TEST_DATABASE_URL requires a disposable database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	root := repositoryRoot(t)
	if err := RunMigrations(ctx, db, filepath.Join(root, "migrations")); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	// Exercise down/up transactionally, without deleting historical records or
	// downgrading schema_migrations in a live/shared database.
	for _, suffix := range []string{"down", "up"} {
		migration, err := os.ReadFile(filepath.Join(root, "migrations/000029_attach_status_history_triggers."+suffix+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		exec(string(migration))
	}
	user, restaurant, order, delivery := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	exec(`INSERT INTO users(user_id,phone,name,role) VALUES($1,$2,'History test','driver')`, user, user.String()[:14])
	exec(`INSERT INTO restaurants(restaurant_id,owner_id,name,location) VALUES($1,$2,'History test',ST_SetSRID(ST_MakePoint(75,24),4326))`, restaurant, user)
	exec(`INSERT INTO orders(order_id,user_id,restaurant_id,status,subtotal,taxes,delivery_fee,total_amount,payment_method) VALUES($1,$2,$3,'order_created',100,5,30,135,'cash_on_delivery')`, order, user, restaurant)
	exec(`INSERT INTO deliveries(delivery_id,order_id,order_created_at,partner_id) SELECT $1,order_id,created_at,$2 FROM orders WHERE order_id=$3`, delivery, user, order)
	exec(`SAVEPOINT transitions`)
	exec(`UPDATE orders SET status='confirmed',updated_by=$1 WHERE order_id=$2`, user, order)
	exec(`UPDATE orders SET status='confirmed',updated_by=$1 WHERE order_id=$2`, user, order) // no-op must not duplicate
	exec(`UPDATE orders SET status='preparing',updated_by=$1 WHERE order_id=$2`, user, order)
	exec(`UPDATE deliveries SET status='en_route_to_restaurant',updated_by=$1 WHERE delivery_id=$2`, user, delivery)
	exec(`UPDATE deliveries SET status='en_route_to_restaurant' WHERE delivery_id=$1`, delivery)
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM order_status_history WHERE order_id=$1 AND changed_by=$2`, order, user).Scan(&count); err != nil || count != 2 {
		t.Fatalf("order actor/history count=%d err=%v", count, err)
	}
	var latest string
	if err := tx.QueryRowContext(ctx, `SELECT to_status::text FROM order_status_history WHERE order_id=$1 ORDER BY changed_at DESC LIMIT 1`, order).Scan(&latest); err != nil || latest != "preparing" {
		t.Fatalf("latest=%s err=%v", latest, err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM delivery_status_history WHERE delivery_id=$1 AND changed_by=$2`, delivery, user).Scan(&count); err != nil || count != 1 {
		t.Fatalf("delivery actor/history count=%d err=%v", count, err)
	}
	exec(`ROLLBACK TO SAVEPOINT transitions`)
	if err := tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM order_status_history WHERE order_id=$1)+(SELECT count(*) FROM delivery_status_history WHERE delivery_id=$2)`, order, delivery).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rolled-back histories=%d err=%v", count, err)
	}
}
