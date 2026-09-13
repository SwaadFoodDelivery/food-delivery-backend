package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	grpcclient "food-delivery-backend/internal/grpc/client"
	"food-delivery-backend/pkg/config"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// Actual HTTP handler -> production typed client -> separately running real
// order-service -> authoritative PostGIS schema. Authentication context is an
// explicit fixture; this isn't another OTP/JWT browser test.
func TestPairedOrderServicePostgres(t *testing.T) {
	dsn, addr, key := os.Getenv("ORDER_RPC_E2E_DATABASE_URL"), os.Getenv("ORDER_RPC_E2E_ADDR"), os.Getenv("ORDER_RPC_E2E_KEY")
	if dsn == "" && addr == "" && key == "" {
		t.Skip("paired order service environment not set")
	}
	u, err := url.Parse(dsn)
	if err != nil || !regexp.MustCompile("^/swaad_grpc_test_[a-z0-9_]+$").MatchString(u.Path) || addr == "" || key == "" {
		t.Fatal("requires complete paired configuration and dedicated swaad_grpc_test_* DB")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var version int
	if err := db.QueryRowContext(ctx, "SELECT version FROM schema_migrations WHERE NOT dirty").Scan(&version); err != nil || version != 29 {
		t.Fatal("requires backend schema29")
	}
	owner, foreign := uuid.New(), uuid.New()
	orderID, foreignOrder := uuid.New(), uuid.New()
	created := time.Now().UTC().Truncate(time.Microsecond)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for _, uid := range []uuid.UUID{owner, foreign} {
		if _, err := tx.ExecContext(ctx, "INSERT INTO users(user_id,phone,name,role,account_status,onboarding_complete) VALUES($1,$2,'Fictional paired RPC client','client','active',true)", uid, uid.String()[:14]); err != nil {
			t.Fatal(err)
		}
	}
	for _, fixture := range []struct{ id, user uuid.UUID }{{orderID, owner}, {foreignOrder, foreign}} {
		if _, err := tx.ExecContext(ctx, "INSERT INTO orders(order_id,created_at,user_id,restaurant_id,status,subtotal,taxes,delivery_fee,total_amount,payment_method) VALUES($1,$2,$3,'10000000-0000-4000-8000-000000000001','confirmed',10.05,0,0,10.05,'cash_on_delivery')", fixture.id, created, fixture.user); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO order_items(order_id,order_created_at,item_id,item_name_snapshot,item_price_snapshot,quantity,line_total) VALUES($1,$2,'40000000-0000-4000-8000-000000000001','Fictional paired stored snapshot',0.05,201,10.05)", orderID, created); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.App.Env = "test"
	cfg.GRPC.OrderAddr = addr
	cfg.GRPC.OrderServiceKey = key
	cfg.GRPC.OrderTimeoutMS = 2000
	reader, err := grpcclient.NewOrderServiceClient(ctx, cfg)
	if err != nil {
		t.Fatal("could not connect paired order service")
	}
	defer reader.Close()
	response := readHTTP(reader, owner.String(), "client", orderID.String())
	if response.Code != 200 {
		t.Fatalf("paired owned read returned %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Data struct {
			ID       string `json:"order_id"`
			Amount   int64  `json:"total_amount_minor"`
			Currency string `json:"currency"`
			Items    []struct {
				Name  string `json:"item_name_snapshot"`
				Price int64  `json:"item_price_snapshot_minor"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.ID != orderID.String() || body.Data.Amount != 1005 || body.Data.Currency != "INR" || len(body.Data.Items) != 1 || body.Data.Items[0].Name != "Fictional paired stored snapshot" || body.Data.Items[0].Price != 5 {
		t.Fatal("paired persisted snapshot mismatch")
	}
	foreignResponse := readHTTP(reader, owner.String(), "client", foreignOrder.String())
	missingResponse := readHTTP(reader, owner.String(), "client", uuid.NewString())
	if foreignResponse.Code != 404 || missingResponse.Code != 404 {
		t.Fatal("paired ownership/missing denial failed")
	}
	if readHTTP(reader, owner.String(), "driver", orderID.String()).Code != 403 {
		t.Fatal("nonclient reader permitted")
	}
	// A wrong service credential must fail closed, not masquerade as user expiry.
	cfg.GRPC.OrderServiceKey = "incorrect-test-service-key-1234567890"
	badReader, err := grpcclient.NewOrderServiceClient(ctx, cfg)
	if err != nil {
		t.Fatal("bad-credential test transport could not connect")
	}
	defer badReader.Close()
	if readHTTP(badReader, owner.String(), "client", orderID.String()).Code != 502 {
		t.Fatal("dependency auth failure did not fail closed")
	}
	var total string
	if err := db.QueryRowContext(ctx, "SELECT total_amount::text FROM orders WHERE order_id=$1 AND created_at=$2", orderID, created).Scan(&total); err != nil || total != "10.05" {
		t.Fatal("read mutated order")
	}
	t.Run("paginated owned summaries", func(t *testing.T) {
		// Three orders share an exact microsecond timestamp; a fourth is one
		// microsecond older. This catches loss of precision and timestamp-only cursors.
		tied := []string{orderID.String(), uuid.NewString(), uuid.NewString()}
		older := uuid.NewString()
		insertOrder := func(id string, at time.Time) {
			t.Helper()
			if _, err := db.ExecContext(ctx, "INSERT INTO orders(order_id,created_at,user_id,restaurant_id,status,subtotal,taxes,delivery_fee,total_amount,payment_method) VALUES($1,$2,$3,'10000000-0000-4000-8000-000000000001','confirmed',10.05,0,0,10.05,'cash_on_delivery')", id, at, owner); err != nil {
				t.Fatal(err)
			}
		}
		for _, id := range tied[1:] {
			insertOrder(id, created)
		}
		insertOrder(older, created.Add(-time.Microsecond))
		sort.Sort(sort.Reverse(sort.StringSlice(tied)))
		type summary struct {
			ID         string  `json:"order_id"`
			Amount     int64   `json:"total_amount_minor"`
			Currency   string  `json:"currency"`
			Restaurant string  `json:"restaurant_name"`
			Created    string  `json:"created_at"`
			Delivery   *string `json:"delivery_status"`
		}
		type envelope struct {
			Data struct {
				Orders []summary `json:"orders"`
				Next   string    `json:"next_cursor"`
			} `json:"data"`
		}
		fetch := func(cursor string) envelope {
			t.Helper()
			rec := listHTTP(reader, owner.String(), "client", "limit=2&cursor="+url.QueryEscape(cursor)+"&user_id="+foreign.String()+"&requester_role=admin")
			if rec.Code != 200 {
				t.Fatalf("paired list status %d: %s", rec.Code, rec.Body.String())
			}
			var out envelope
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			return out
		}
		first := fetch("")
		if len(first.Data.Orders) != 2 || first.Data.Next == "" || len(first.Data.Next) > 1024 || first.Data.Orders[0].ID != tied[0] || first.Data.Orders[1].ID != tied[1] {
			t.Fatal("first page/tie ordering failed")
		}
		for _, row := range first.Data.Orders {
			at, err := time.Parse(time.RFC3339Nano, row.Created)
			if err != nil || !at.Equal(created) || row.Amount != 1005 || row.Currency != "INR" || row.Restaurant == "" || row.Delivery != nil {
				t.Fatal("list summary contract or precision lost")
			}
		}
		// A newer insertion cannot displace or duplicate entries on the next page.
		insertOrder(uuid.NewString(), created.Add(time.Second))
		second := fetch(first.Data.Next)
		if len(second.Data.Orders) != 2 || second.Data.Next != "" || second.Data.Orders[0].ID != tied[2] || second.Data.Orders[1].ID != older {
			t.Fatal("continuation skipped/duplicated rows or fabricated next page")
		}
		at, err := time.Parse(time.RFC3339Nano, second.Data.Orders[1].Created)
		if err != nil || !at.Equal(created.Add(-time.Microsecond)) {
			t.Fatal("older microsecond was lost")
		}
		if rec := listHTTP(reader, foreign.String(), "client", "limit=2&cursor="+url.QueryEscape(first.Data.Next)); rec.Code != 400 {
			t.Fatalf("foreign cursor status %d", rec.Code)
		}
		for _, cursor := range []string{"malformed", "!" + first.Data.Next[1:]} {
			if rec := listHTTP(reader, owner.String(), "client", "limit=2&cursor="+url.QueryEscape(cursor)); rec.Code != 400 {
				t.Fatalf("tampered cursor status %d", rec.Code)
			}
		}
		empty := uuid.New()
		if _, err := db.ExecContext(ctx, "INSERT INTO users(user_id,phone,name,role,account_status,onboarding_complete) VALUES($1,$2,'Fictional empty list client','client','active',true)", empty, empty.String()[:14]); err != nil {
			t.Fatal(err)
		}
		if rec := listHTTP(reader, empty.String(), "client", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"orders":[]`) || !strings.Contains(rec.Body.String(), `"next_cursor":""`) {
			t.Fatal("empty paired list contract failed")
		}
		if listHTTP(reader, owner.String(), "driver", "").Code != 403 || listHTTP(badReader, owner.String(), "client", "").Code != 502 {
			t.Fatal("list authorization did not fail closed")
		}
	})
	t.Log("paired HTTP -> real order-service -> PostGIS passed: owned snapshot, foreign/missing, role denial, dependency auth failure; fictional fixtures retained")
}
