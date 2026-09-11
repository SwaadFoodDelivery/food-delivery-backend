package repository

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	postgresinfra "food-delivery-backend/infra/postgres"
	"food-delivery-backend/internal/services/delivery/models"
	orderrepo "food-delivery-backend/internal/services/order/repository"
	paymentmodels "food-delivery-backend/internal/services/payment/models"
	paymentrepo "food-delivery-backend/internal/services/payment/repository"
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
	t.Run("customer cancellation and legacy current status are readable", func(t *testing.T) {
		id, _ := newOrder("upi")
		orders := orderrepo.NewPostgresRepository(db)
		userID := uuid.MustParse("00000000-0000-4000-8000-000000000004")
		if _, err := orders.CancelForUser(ctx, userID, id); err != nil {
			t.Fatal(err)
		}
		history, err := orders.GetHistory(ctx, userID, id)
		if err != nil || history.Status != "cancelled" || len(history.OrderStatus) != 1 || history.OrderStatus[0].ToStatus != "cancelled" {
			t.Fatalf("cancellation not persisted: %+v %v", history, err)
		}
		exec(`DELETE FROM order_status_history WHERE order_id=$1`, id) // simulate a pre-29 order, only this test fixture
		history, err = orders.GetHistory(ctx, userID, id)
		if err != nil || history.Status != "cancelled" || len(history.OrderStatus) != 0 {
			t.Fatalf("legacy state missing or events fabricated: %+v %v", history, err)
		}
		if _, err := orders.GetHistory(ctx, uuid.New(), id); err != orderrepo.ErrOrderNotFound {
			t.Fatalf("history leaked to another customer: %v", err)
		}
	})
	// Retries with different keys must not create concurrent charge attempts or
	// charge an already-paid order after the original HTTP response was lost.
	retryID, retryCreated := newOrder("upi")
	exec(`UPDATE orders SET subtotal=100.05,total_amount=136.05 WHERE order_id=$1`, retryID)
	payments := paymentrepo.NewPostgresRepository(db)
	input := paymentmodels.Payment{OrderID: retryID, CreatedAt: retryCreated, UserID: uuid.MustParse("00000000-0000-4000-8000-000000000004"), Amount: 13605, Provider: "mock"}
	type attempt struct {
		payment paymentmodels.Payment
		replay  bool
		err     error
	}
	results := make(chan attempt, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, replay, err := payments.CreatePending(ctx, input, uuid.NewString())
			results <- attempt{p, replay, err}
		}()
	}
	wg.Wait()
	close(results)
	var paymentID uuid.UUID
	createdAttempts := 0
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		if !result.replay {
			createdAttempts++
		}
		if paymentID == uuid.Nil {
			paymentID = result.payment.PaymentID
		}
		if paymentID != result.payment.PaymentID {
			t.Fatal("concurrent requests created different payments")
		}
	}
	if createdAttempts != 1 {
		t.Fatalf("created %d charge attempts", createdAttempts)
	}
	paid, err := payments.Complete(ctx, paymentID, paymentmodels.StatusSuccess, "test_"+uuid.NewString(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if paid.Amount != 13605 {
		t.Fatalf("fractional amount changed: %d", paid.Amount)
	}
	again, replay, err := payments.CreatePending(ctx, input, uuid.NewString())
	if err != nil || !replay || again.PaymentID != paymentID || again.Status != paymentmodels.StatusSuccess {
		t.Fatalf("lost response retry=%+v replay=%v err=%v", again, replay, err)
	}
	t.Run("interrupted mock attempt expires without reviving or duplicating it", func(t *testing.T) {
		orderID, orderCreated := newOrder("upi")
		in := paymentmodels.Payment{OrderID: orderID, CreatedAt: orderCreated, UserID: input.UserID, Amount: 13600, Provider: "mock"}
		key := uuid.NewString()
		old, _, err := payments.CreatePending(ctx, in, key)
		if err != nil {
			t.Fatal(err)
		}
		// Fresh pending work is still protected against another charge.
		fresh, replay, err := payments.CreatePending(ctx, in, uuid.NewString())
		if err != nil || !replay || fresh.PaymentID != old.PaymentID || fresh.Status != paymentmodels.StatusPending {
			t.Fatalf("fresh pending attempt lost: %+v replay=%v err=%v", fresh, replay, err)
		}
		exec(`UPDATE payments SET updated_at=NOW()-INTERVAL '2 minutes' WHERE payment_id=$1`, old.PaymentID)
		expired, replay, err := payments.CreatePending(ctx, in, key)
		if err != nil || !replay || expired.PaymentID != old.PaymentID || expired.FailureCode != "PAYMENT_INTERRUPTED" || expired.Status != paymentmodels.StatusFailed {
			t.Fatalf("expired key changed identity/outcome: %+v replay=%v err=%v", expired, replay, err)
		}
		// A delayed original completion cannot overwrite the expired outcome.
		late, err := payments.Complete(ctx, old.PaymentID, paymentmodels.StatusSuccess, "late_mock", "", "")
		if err != nil || late.Status != paymentmodels.StatusFailed {
			t.Fatalf("late completion revived abandoned attempt: %+v %v", late, err)
		}
		results := make(chan attempt, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				p, replay, err := payments.CreatePending(ctx, in, uuid.NewString())
				results <- attempt{p, replay, err}
			}()
		}
		wg.Wait()
		close(results)
		created := 0
		var next uuid.UUID
		for result := range results {
			if result.err != nil || result.payment.Status != paymentmodels.StatusPending || result.payment.PaymentID == old.PaymentID {
				t.Fatalf("recovery=%+v", result)
			}
			if !result.replay {
				created++
			}
			if next == uuid.Nil {
				next = result.payment.PaymentID
			}
			if result.payment.PaymentID != next {
				t.Fatal("recovery created duplicate attempts")
			}
		}
		if created != 1 {
			t.Fatalf("created %d recovery attempts", created)
		}
	})
	t.Run("non-mock pending attempts are never guessed failed", func(t *testing.T) {
		orderID, orderCreated := newOrder("upi")
		in := paymentmodels.Payment{OrderID: orderID, CreatedAt: orderCreated, UserID: input.UserID, Amount: 13600, Provider: "requires-reconciliation"}
		old, _, err := payments.CreatePending(ctx, in, uuid.NewString())
		if err != nil {
			t.Fatal(err)
		}
		exec(`UPDATE payments SET updated_at=NOW()-INTERVAL '2 minutes' WHERE payment_id=$1`, old.PaymentID)
		again, replay, err := payments.CreatePending(ctx, in, uuid.NewString())
		if err != nil || !replay || again.Status != paymentmodels.StatusPending || again.PaymentID != old.PaymentID {
			t.Fatalf("unsafe expiry of non-mock payment: %+v replay=%v err=%v", again, replay, err)
		}
	})
	// Only generated test orders are removed, never the seed or dev records.
	for _, orderID := range []uuid.UUID{id, cod, cancelled} {
		exec(`DELETE FROM payments WHERE order_id=$1`, orderID)
		exec(`DELETE FROM deliveries WHERE order_id=$1`, orderID)
		exec(`DELETE FROM orders WHERE order_id=$1`, orderID)
	}
}
