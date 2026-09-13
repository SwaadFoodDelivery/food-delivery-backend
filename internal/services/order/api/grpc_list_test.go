package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/constants"
	"food-delivery-backend/pkg/config"
	orderpb "github.com/SwaadFoodDelivery/proto/order"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type listRPC struct {
	orderpb.UnimplementedOrderServiceServer
	calls  atomic.Int32
	seen   chan *orderpb.GetUserOrdersRequest
	result *orderpb.GetUserOrdersResponse
	code   codes.Code
	wait   bool
}

func (s *listRPC) GetUserOrders(ctx context.Context, in *orderpb.GetUserOrdersRequest) (*orderpb.GetUserOrdersResponse, error) {
	s.calls.Add(1)
	md, _ := metadata.FromIncomingContext(ctx)
	if keys := md.Get("x-order-service-key"); len(keys) != 1 || keys[0] != grpcKey {
		return nil, status.Error(codes.Unauthenticated, "sensitive service key diagnostic")
	}
	if s.wait {
		<-ctx.Done()
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	if s.code != codes.OK {
		return nil, status.Error(s.code, "sensitive internal diagnostic")
	}
	if s.seen != nil {
		s.seen <- in
	}
	if s.result == nil {
		return &orderpb.GetUserOrdersResponse{}, nil
	}
	return s.result, nil
}

func listHTTP(reader OrderLister, uid, role, query string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constants.AuthContextUserIDKey, uid)
		c.Set(constants.AuthContextRoleKey, role)
	})
	router.GET("/orders", NewGRPCListHandler(reader))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orders?"+query, nil))
	return rec
}

func TestHTTPListTypedRPCIdentityAndSummary(t *testing.T) {
	service := &listRPC{seen: make(chan *orderpb.GetUserOrdersRequest, 1), result: &orderpb.GetUserOrdersResponse{
		Orders: []*orderpb.OrderResponse{{OrderId: grpcOrder, Status: orderpb.OrderStatus_CONFIRMED, RestaurantName: "Demo kitchen", TotalAmountMinor: 1005, Currency: "INR", PaymentMethod: "cod", CreatedAt: "2026-09-13T10:00:00.123456Z", DeliveryStatus: "pending"}}, NextCursor: "opaque-cursor",
	}}
	reader := rpcReader(t, service, grpcKey)
	rec := listHTTP(reader, grpcUser, "client", "limit=1&cursor=previous&user_id=attacker&requester_role=admin")
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	in := <-service.seen
	if in.UserId != grpcUser || in.RequesterRole != "client" || in.Limit != 1 || in.Cursor != "previous" {
		t.Fatalf("unexpected delegation: %+v", in)
	}
	var body struct {
		Data struct {
			Orders []map[string]any `json:"orders"`
			Next   string           `json:"next_cursor"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Orders) != 1 || body.Data.Next != "opaque-cursor" {
		t.Fatal("pagination envelope missing")
	}
	row := body.Data.Orders[0]
	for key, want := range map[string]any{"order_id": grpcOrder, "status": "confirmed", "restaurant_name": "Demo kitchen", "total_amount_minor": float64(1005), "currency": "INR", "payment_method": "cod", "created_at": "2026-09-13T10:00:00.123456Z", "delivery_status": "pending"} {
		if row[key] != want {
			t.Fatalf("%s got %v want %v", key, row[key], want)
		}
	}
	if _, ok := row["items"]; ok {
		t.Fatal("list unexpectedly hydrates items")
	}
	if strings.Contains(rec.Body.String(), `"total":`) {
		t.Fatal("unknown total exposed")
	}
}

func TestHTTPListDefaultEmptyAndOptionalDelivery(t *testing.T) {
	service := &listRPC{seen: make(chan *orderpb.GetUserOrdersRequest, 1)}
	reader := rpcReader(t, service, grpcKey)
	rec := listHTTP(reader, grpcUser, "client", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"orders":[]`) || !strings.Contains(rec.Body.String(), `"next_cursor":""`) {
		t.Fatalf("empty contract: %s", rec.Body.String())
	}
	if (<-service.seen).Limit != 20 {
		t.Fatal("default limit changed")
	}
	service.result = &orderpb.GetUserOrdersResponse{Orders: []*orderpb.OrderResponse{{OrderId: grpcOrder}}}
	rec = listHTTP(reader, grpcUser, "client", "limit=50")
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "delivery_status") {
		t.Fatal("missing delivery is not omitted")
	}
}

func TestHTTPListRejectsBeforeRPC(t *testing.T) {
	service := &listRPC{}
	reader := rpcReader(t, service, grpcKey)
	for _, tc := range []struct {
		uid, role, query string
		want             int
	}{
		{"", "client", "", 401}, {"00000000-0000-0000-0000-000000000000", "client", "", 401},
		{grpcUser, "driver", "", 403}, {grpcUser, "admin", "", 403},
		{grpcUser, "client", "limit=0", 400}, {grpcUser, "client", "limit=-1", 400},
		{grpcUser, "client", "limit=51", 400}, {grpcUser, "client", "limit=2147483648", 400},
		{grpcUser, "client", "limit=oops", 400}, {grpcUser, "client", "cursor=" + strings.Repeat("x", 1025), 400},
	} {
		if rec := listHTTP(reader, tc.uid, tc.role, tc.query); rec.Code != tc.want {
			t.Fatalf("%s status %d want %d", tc.query, rec.Code, tc.want)
		}
	}
	if service.calls.Load() != 0 {
		t.Fatal("invalid input reached RPC")
	}
}

func TestHTTPListRPCFailuresAreSanitizedAndFailClosed(t *testing.T) {
	for code, want := range map[codes.Code]int{codes.InvalidArgument: 400, codes.PermissionDenied: 403, codes.DeadlineExceeded: 504, codes.Unavailable: 503, codes.Canceled: 503, codes.Unauthenticated: 502, codes.Internal: 502, codes.Unimplemented: 502} {
		t.Run(code.String(), func(t *testing.T) {
			service := &listRPC{code: code}
			rec := listHTTP(rpcReader(t, service, grpcKey), grpcUser, "client", "")
			if rec.Code != want || strings.Contains(rec.Body.String(), "sensitive") || service.calls.Load() != 1 {
				t.Fatalf("failure mapping: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
	service := &listRPC{}
	rec := listHTTP(rpcReader(t, service, strings.Repeat("x", 32)), grpcUser, "client", "")
	if rec.Code != 502 {
		t.Fatal("dependency auth must not request customer re-login")
	}
	service = &listRPC{wait: true}
	reader := rpcReader(t, service, grpcKey)
	start := time.Now()
	rec = listHTTP(reader, grpcUser, "client", "")
	if rec.Code != 504 || time.Since(start) > time.Second || service.calls.Load() != 1 {
		t.Fatal("deadline/fail-closed violation")
	}
}

func TestLocalListRejectsCursorWithoutServiceCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/orders", NewHandler(nil).List)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orders?cursor=previous", nil))
	if rec.Code != 400 {
		t.Fatalf("local cursor silently ignored: %d", rec.Code)
	}
}

func TestRegisteredListRouteUsesOnlySelectedDependency(t *testing.T) {
	for _, tc := range []struct {
		name             string
		enabled, missing bool
		want             int
		calls            int32
	}{
		{"enabled", true, false, 200, 1},
		{"disabled cursor", false, false, 400, 0},
		{"enabled missing client", true, true, 503, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &listRPC{}
			reader := rpcReader(t, service, grpcKey)
			if tc.missing {
				reader = nil
			}
			cfg := &config.Config{}
			cfg.GRPC.OrderRequired = tc.enabled
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(constants.AuthContextUserIDKey, grpcUser)
				c.Set(constants.AuthContextRoleKey, "client")
			})
			// DB deliberately absent: a hidden local fallback cannot pass this test.
			RegisterRoutes(router.Group("/api/v1"), &app.Container{Config: cfg, OrderClient: reader})
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/orders?cursor=previous", nil))
			if rec.Code != tc.want || service.calls.Load() != tc.calls {
				t.Fatalf("route selection: status %d calls %d", rec.Code, service.calls.Load())
			}
		})
	}
}
