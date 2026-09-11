package repository

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	postgresinfra "food-delivery-backend/infra/postgres"
	"food-delivery-backend/internal/services/delivery/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func TestPaymentDeliveryGatePostgres(t *testing.T) {
	dsn := os.Getenv("PAYMENT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PAYMENT_TEST_DATABASE_URL requires a disposable _test database")
	}
	u, err := url.Parse(dsn)
	if err != nil || (!strings.Contains(u.Path, "_test") && !(u.Path == "/food_delivery" && os.Getenv("GITHUB_ACTIONS") == "true")) {
		t.Fatal("refusing non-test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db, err := sqlx.ConnectContext(ctx, "postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "../../../..")
	if err := postgresinfra.RunMigrations(ctx, db.DB, filepath.Join(root, "migrations")); err != nil {
		t.Fatal(err)
	}
	seed, err := os.ReadFile(filepath.Join(root, "scripts/seed_demo.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(seed)); err != nil {
		t.Fatal(err)
	}
	repo := NewPostgresRepository(db)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	newOrder := func(method string) (uuid.UUID, time.Time) {
		t.Helper()
		id := uuid.New()
		var created time.Time
		err := db.GetContext(ctx, &created, `INSERT INTO orders (order_id,user_id,restaurant_id,status,subtotal,taxes,delivery_fee,total_amount,payment_method)
		VALUES ($1,'00000000-0000-4000-8000-000000000004','10000000-0000-4000-8000-000000000001','order_created',100,6,30,136,$2) RETURNING created_at`, id, method)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			for _, table := range []string{"payments", "deliveries", "orders"} {
				if _, err := db.ExecContext(context.Background(), "DELETE FROM "+table+" WHERE order_id=$1", id); err != nil {
					t.Errorf("cleanup %s: %v", table, err)
				}
			}
		})
		return id, created
	}
	count := func(id uuid.UUID, want int) {
		t.Helper()
		var got int
		if err := db.GetContext(ctx, &got, `SELECT count(*) FROM deliveries WHERE order_id=$1`, id); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("deliveries=%d want=%d", got, want)
		}
	}
	id, created := newOrder("upi")
	if _, err := repo.EnsureMockDelivery(ctx, id, created, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	count(id, 0)
	exec(`INSERT INTO payments (order_id,order_created_at,user_id,amount,status,provider,idempotency_key) VALUES ($1,$2,'00000000-0000-4000-8000-000000000004',136,'failed','mock',$3)`, id, created, uuid.NewString())
	if err := repo.AdvanceMockDeliveries(ctx, time.Now().Add(time.Hour), 30*time.Second); err != nil {
		t.Fatal(err)
	}
	count(id, 0)
	// A crash after successful payment is recovered from database state alone.
	exec(`UPDATE payments SET status='success' WHERE order_id=$1`, id)
	if err := repo.AdvanceMockDeliveries(ctx, time.Now(), 30*time.Second); err != nil {
		t.Fatal(err)
	}
	count(id, 1)
	var driver uuid.UUID
	if err := db.GetContext(ctx, &driver, `SELECT partner_id FROM deliveries WHERE order_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateForDriver(ctx, driver, models.StatusEnRouteToRestaurant, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.EnsureMockDelivery(ctx, id, created, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := db.GetContext(ctx, &status, `SELECT status::text FROM orders WHERE order_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if status != "preparing" {
		t.Fatalf("replay reset progressed order to %s", status)
	}
	// Legacy delivery rows for failed payments may neither auto-advance nor
	// be manually progressed through a driver's authenticated endpoint.
	exec(`UPDATE payments SET status='failed' WHERE order_id=$1`, id)
	if err := repo.AdvanceMockDeliveries(ctx, time.Now().Add(time.Hour), 30*time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateForDriver(ctx, driver, models.StatusArrivedAtRestaurant, 30*time.Second); err != ErrDeliveryNotFound {
		t.Fatalf("unpaid manual advancement error=%v", err)
	}
	if err := db.GetContext(ctx, &status, `SELECT status::text FROM orders WHERE order_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if status != "preparing" {
		t.Fatalf("unpaid worker advanced order to %s", status)
	}
	exec(`UPDATE payments SET status='success' WHERE order_id=$1`, id)
	if err := repo.AdvanceMockDeliveries(ctx, time.Now().Add(time.Hour), 30*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := db.GetContext(ctx, &status, `SELECT status::text FROM orders WHERE order_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if status != "delivered" {
		t.Fatalf("paid delivery did not finish: %s", status)
	}
	cod, codCreated := newOrder("cash_on_delivery")
	if _, err := repo.EnsureMockDelivery(ctx, cod, codCreated, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	count(cod, 1)
	cancelled, cancelledCreated := newOrder("cash_on_delivery")
	exec(`UPDATE orders SET status='cancelled' WHERE order_id=$1`, cancelled)
	if _, err := repo.EnsureMockDelivery(ctx, cancelled, cancelledCreated, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	count(cancelled, 0)
	// Only generated test orders are removed, never the seed or dev records.
	for _, orderID := range []uuid.UUID{id, cod, cancelled} {
		exec(`DELETE FROM payments WHERE order_id=$1`, orderID)
		exec(`DELETE FROM deliveries WHERE order_id=$1`, orderID)
		exec(`DELETE FROM orders WHERE order_id=$1`, orderID)
	}
}
