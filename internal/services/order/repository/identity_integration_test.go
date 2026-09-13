package repository

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"regexp"
	"testing"
	"time"

	ordermodels "food-delivery-backend/internal/services/order/models"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Root provisions/migrates this disposable database. This test only adds private
// fictional fixtures and exercises the production history/cancellation paths.
func TestOrderIdentityPostgres(t *testing.T) {
	dsn := os.Getenv("ORDER_IDENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ORDER_IDENTITY_TEST_DATABASE_URL not set")
	}
	databaseName, err := identityDatabaseName(dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := sqlx.ConnectContext(ctx, "postgres", dsn)
	if err != nil {
		t.Fatal("could not connect to dedicated identity test database")
	}
	defer db.Close()
	var actual string
	if err := db.GetContext(ctx, &actual, "SELECT current_database()"); err != nil || actual != databaseName {
		t.Fatal("resolved database differs from guarded name")
	}
	var version int
	if err := db.GetContext(ctx, &version, "SELECT version FROM schema_migrations WHERE NOT dirty"); err != nil || version != 29 {
		t.Fatal("requires clean backend schema29")
	}
	var postgis string
	if err := db.GetContext(ctx, &postgis, "SELECT postgis_version()"); err != nil {
		t.Fatal("requires actual PostGIS")
	}
	repo := NewPostgresRepository(db)
	baseTime := time.Now().UTC().Truncate(time.Microsecond)
	newUser := func(t *testing.T) uuid.UUID {
		t.Helper()
		id := uuid.New()
		if _, err := db.ExecContext(ctx, `INSERT INTO users(user_id,phone,name,role) VALUES($1,$2,'Fictional identity guard client','client')`, id, id.String()[:14]); err != nil {
			t.Fatal(err)
		}
		return id
	}
	insertOrder := func(id, owner uuid.UUID, at time.Time, state string) error {
		_, err := db.ExecContext(ctx, `INSERT INTO orders(order_id,created_at,user_id,restaurant_id,status,subtotal,taxes,delivery_fee,total_amount,payment_method)
			VALUES($1,$2,$3,'10000000-0000-4000-8000-000000000001',$4,10.05,0,0,10.05,'cash_on_delivery')`, id, at, owner, state)
		return err
	}
	addOrder := func(t *testing.T, id, owner uuid.UUID, at time.Time, state string) {
		t.Helper()
		if err := insertOrder(id, owner, at, state); err != nil {
			t.Fatal(err)
		}
	}
	addDelivery := func(t *testing.T, order, owner uuid.UUID, at time.Time) uuid.UUID {
		t.Helper()
		id := uuid.New()
		if _, err := db.ExecContext(ctx, `INSERT INTO deliveries(delivery_id,order_id,order_created_at,partner_id,status,next_transition_at)
			VALUES($1,$2,$3,$4,'assigned',$5)`, id, order, at, owner, at.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		return id
	}

	t.Run("ambiguous leaves all side effects unchanged", func(t *testing.T) {
		owner, order := newUser(t), uuid.New()
		addOrder(t, order, owner, baseTime, "confirmed")
		addOrder(t, order, owner, baseTime.Add(time.Microsecond), "delivered")
		addDelivery(t, order, owner, baseTime)
		before := identitySnapshot(t, ctx, db, owner, order)
		if _, err := repo.GetHistory(ctx, owner, order); !errors.Is(err, ErrOrderAmbiguous) {
			t.Fatalf("history error = %v", err)
		}
		if _, err := repo.CancelForUser(ctx, owner, order); !errors.Is(err, ErrOrderAmbiguous) {
			t.Fatalf("cancellation error = %v", err)
		}
		if after := identitySnapshot(t, ctx, db, owner, order); after != before {
			t.Fatal("ambiguous request changed order/delivery/history/audit/notification data")
		}
	})

	t.Run("single cancellation history and terminal replay", func(t *testing.T) {
		owner, order := newUser(t), uuid.New()
		addOrder(t, order, owner, baseTime, "confirmed")
		delivery := addDelivery(t, order, owner, baseTime)
		if _, err := db.ExecContext(ctx, `INSERT INTO order_status_history(order_id,order_created_at,from_status,to_status,changed_by) VALUES($1,$2,'order_created','confirmed',$3)`, order, baseTime, owner); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO delivery_status_history(delivery_id,to_status,changed_by) VALUES($1,'assigned',$2)`, delivery, owner); err != nil {
			t.Fatal(err)
		}
		history, err := repo.GetHistory(ctx, owner, order)
		if err != nil || history.Status != "confirmed" || len(history.OrderStatus) != 1 || len(history.DeliveryStatus) != 1 {
			t.Fatalf("single history = %+v, %v", history, err)
		}
		out, err := repo.CancelForUser(ctx, owner, order)
		if err != nil || out.Status != "cancelled" || !out.CreatedAt.Equal(baseTime) || out.OrderID != order {
			t.Fatalf("single cancellation = %+v, %v", out, err)
		}
		var next sql.NullTime
		if err := db.GetContext(ctx, &next, "SELECT next_transition_at FROM deliveries WHERE delivery_id=$1", delivery); err != nil || next.Valid {
			t.Fatal("cancelled delivery is still scheduled")
		}
		history, err = repo.GetHistory(ctx, owner, order)
		if err != nil || history.Status != "cancelled" || len(history.OrderStatus) != 2 || history.OrderStatus[1].ToStatus != "cancelled" {
			t.Fatalf("persisted cancellation history = %+v, %v", history, err)
		}
		for _, query := range []string{
			"SELECT count(*) FROM audit_logs WHERE actor_id=$1 AND action='order_cancelled'",
			"SELECT count(*) FROM notifications WHERE recipient_id=$1 AND title='Order cancelled'",
		} {
			var count int
			if err := db.GetContext(ctx, &count, query, owner); err != nil || count != 1 {
				t.Fatal("cancellation did not produce exactly one audit/notification")
			}
		}
		before := identitySnapshot(t, ctx, db, owner, order)
		if _, err := repo.CancelForUser(ctx, owner, order); !errors.Is(err, ErrOrderNotCancelable) {
			t.Fatalf("repeat cancellation error = %v", err)
		}
		if identitySnapshot(t, ctx, db, owner, order) != before {
			t.Fatal("terminal replay changed data")
		}
	})

	t.Run("all terminal states denied", func(t *testing.T) {
		for _, state := range []string{"cancelled", "rejected", "delivered"} {
			owner, order := newUser(t), uuid.New()
			addOrder(t, order, owner, baseTime, state)
			before := identitySnapshot(t, ctx, db, owner, order)
			if _, err := repo.CancelForUser(ctx, owner, order); !errors.Is(err, ErrOrderNotCancelable) {
				t.Fatalf("%s cancellation error = %v", state, err)
			}
			if identitySnapshot(t, ctx, db, owner, order) != before {
				t.Fatal("terminal denial changed data")
			}
		}
	})

	t.Run("foreign and missing are indistinguishable", func(t *testing.T) {
		owner, foreign, order := newUser(t), newUser(t), uuid.New()
		addOrder(t, order, foreign, baseTime, "confirmed")
		addOrder(t, order, foreign, baseTime.Add(time.Microsecond), "confirmed")
		before := identitySnapshot(t, ctx, db, foreign, order)
		for _, id := range []uuid.UUID{order, uuid.New()} {
			if _, err := repo.GetHistory(ctx, owner, id); !errors.Is(err, ErrOrderNotFound) {
				t.Fatalf("foreign/missing history error = %v", err)
			}
			if _, err := repo.CancelForUser(ctx, owner, id); !errors.Is(err, ErrOrderNotFound) {
				t.Fatalf("foreign/missing cancellation error = %v", err)
			}
		}
		if identitySnapshot(t, ctx, db, foreign, order) != before {
			t.Fatal("foreign request changed data")
		}
	})

	t.Run("shared UUID with different owners stays isolated", func(t *testing.T) {
		owner, foreign, order := newUser(t), newUser(t), uuid.New()
		foreignAt := baseTime.Add(time.Microsecond)
		addOrder(t, order, owner, baseTime, "confirmed")
		addOrder(t, order, foreign, foreignAt, "preparing")
		addOrder(t, order, foreign, foreignAt.Add(time.Microsecond), "delivered")
		foreignDelivery := addDelivery(t, order, foreign, foreignAt)
		if _, err := db.ExecContext(ctx, `INSERT INTO order_status_history(order_id,order_created_at,to_status,changed_by) VALUES($1,$2,'preparing',$3)`, order, foreignAt, foreign); err != nil {
			t.Fatal(err)
		}
		history, err := repo.GetHistory(ctx, owner, order)
		if err != nil || history.Status != "confirmed" || len(history.OrderStatus) != 0 || len(history.DeliveryStatus) != 0 {
			t.Fatal("foreign same-ID rows affected history resolution")
		}
		if _, err := repo.CancelForUser(ctx, owner, order); err != nil {
			t.Fatalf("foreign rows incorrectly caused ambiguity: %v", err)
		}
		var state string
		if err := db.GetContext(ctx, &state, "SELECT status::text FROM orders WHERE order_id=$1 AND created_at=$2", order, foreignAt); err != nil || state != "preparing" {
			t.Fatal("foreign same-ID row was modified")
		}
		var next sql.NullTime
		if err := db.GetContext(ctx, &next, "SELECT next_transition_at FROM deliveries WHERE delivery_id=$1", foreignDelivery); err != nil || !next.Valid {
			t.Fatal("foreign composite delivery was modified")
		}
		var count int
		if err := db.GetContext(ctx, &count, "SELECT count(*) FROM notifications WHERE recipient_id=$1", foreign); err != nil || count != 0 {
			t.Fatal("foreign user received cancellation side effects")
		}
	})

	t.Run("insert after selection cannot be cancelled", func(t *testing.T) {
		owner, order := newUser(t), uuid.New()
		addOrder(t, order, owner, baseTime, "confirmed")
		selected := make(chan struct{})
		resume := make(chan error, 1)
		done := make(chan struct {
			out ordermodels.HistoryItem
			err error
		}, 1)
		go func() {
			out, err := repo.cancelForUser(ctx, owner, order, func() error {
				close(selected)
				select {
				case err := <-resume:
					return err
				case <-ctx.Done():
					return ctx.Err()
				}
			})
			done <- struct {
				out ordermodels.HistoryItem
				err error
			}{out, err}
		}()
		select {
		case <-selected:
		case early := <-done:
			t.Fatalf("cancellation ended before selection: %v", early.err)
		case <-ctx.Done():
			t.Fatal("selection deadline exceeded")
		}
		// This commits on another connection while cancellation holds its row lock.
		newAt := baseTime.Add(time.Microsecond)
		insertErr := insertOrder(order, owner, newAt, "confirmed")
		resume <- insertErr
		var result struct {
			out ordermodels.HistoryItem
			err error
		}
		select {
		case result = <-done:
		case <-ctx.Done():
			t.Fatal("cancellation deadline exceeded")
		}
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		if result.err != nil || !result.out.CreatedAt.Equal(baseTime) {
			t.Fatalf("cancellation result = %+v, %v", result.out, result.err)
		}
		var states []struct {
			CreatedAt time.Time `db:"created_at"`
			Status    string    `db:"status"`
		}
		if err := db.SelectContext(ctx, &states, "SELECT created_at,status::text FROM orders WHERE order_id=$1 AND user_id=$2 ORDER BY created_at", order, owner); err != nil {
			t.Fatal(err)
		}
		if len(states) != 2 || states[0].Status != "cancelled" || states[1].Status != "confirmed" {
			t.Fatal("newly inserted partition was cancelled")
		}
		var newHistory int
		if err := db.GetContext(ctx, &newHistory, "SELECT count(*) FROM order_status_history WHERE order_id=$1 AND order_created_at=$2", order, newAt); err != nil || newHistory != 0 {
			t.Fatal("new partition received status history")
		}
		if _, err := repo.GetHistory(ctx, owner, order); !errors.Is(err, ErrOrderAmbiguous) {
			t.Fatal("subsequent history did not detect new ambiguity")
		}
	})
	t.Log("verified ambiguity is side-effect-free, ownership isolation, exact partition cancellation, and deterministic concurrent insert protection")
}

func identityDatabaseName(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" || !regexp.MustCompile(`^/swaad_grpc_test_[a-z0-9_]+$`).MatchString(u.Path) {
		return "", errors.New("identity integration requires a dedicated swaad_grpc_test_* database URL")
	}
	// Do not allow libpq query parameters to override the guarded database path.
	if u.Query().Has("dbname") || u.Query().Has("database") {
		return "", errors.New("database-name overrides are forbidden")
	}
	return u.Path[1:], nil
}

func TestIdentityDatabaseGuard(t *testing.T) {
	for _, dsn := range []string{"", "postgres://localhost/food_delivery", "postgres://localhost/swaad_e2e", "postgres://localhost/swaad_grpc_test_ok?dbname=food_delivery", "postgres://localhost/swaad_grpc_test_ok?database=food_delivery", "postgres://localhost/swaad_grpc_test_", "file:///swaad_grpc_test_ok"} {
		if _, err := identityDatabaseName(dsn); err == nil {
			t.Fatal("accepted unsafe identity database target")
		}
	}
	if name, err := identityDatabaseName("postgres://127.0.0.1:5432/swaad_grpc_test_20260912?sslmode=disable"); err != nil || name != "swaad_grpc_test_20260912" {
		t.Fatal("dedicated database guard rejected valid target")
	}
}

type identityState struct{ orders, deliveries, history, audit, notifications string }

func identitySnapshot(t *testing.T, ctx context.Context, db *sqlx.DB, owner, order uuid.UUID) identityState {
	t.Helper()
	var state identityState
	for _, query := range []struct {
		sql    string
		args   []any
		result *string
	}{
		{`SELECT COALESCE(jsonb_agg(to_jsonb(o) ORDER BY o.created_at),'[]')::text FROM orders o WHERE order_id=$1`, []any{order}, &state.orders},
		{`SELECT COALESCE(jsonb_agg(to_jsonb(d) ORDER BY d.delivery_id),'[]')::text FROM deliveries d WHERE order_id=$1`, []any{order}, &state.deliveries},
		{`SELECT COALESCE(jsonb_agg(to_jsonb(h) ORDER BY h.history_id),'[]')::text FROM order_status_history h WHERE order_id=$1`, []any{order}, &state.history},
		{`SELECT COALESCE(jsonb_agg(to_jsonb(a) ORDER BY a.audit_id),'[]')::text FROM audit_logs a WHERE actor_id=$1`, []any{owner}, &state.audit},
		{`SELECT COALESCE(jsonb_agg(to_jsonb(n) ORDER BY n.notification_id),'[]')::text FROM notifications n WHERE recipient_id=$1`, []any{owner}, &state.notifications},
	} {
		if err := db.GetContext(ctx, query.result, query.sql, query.args...); err != nil {
			t.Fatal(err)
		}
	}
	return state
}
