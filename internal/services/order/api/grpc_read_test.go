package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/constants"
	grpcclient "food-delivery-backend/internal/grpc/client"
	"food-delivery-backend/pkg/config"
	orderpb "github.com/SwaadFoodDelivery/proto/order"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const grpcUser = "00000000-0000-4000-8000-000000000001"
const grpcOrder = "00000000-0000-4000-8000-000000000002"
const grpcKey = "fictional-local-service-key-1234567890"

type readRPC struct {
	orderpb.UnimplementedOrderServiceServer
	calls      atomic.Int32
	seen       chan *orderpb.GetOrderRequest
	resultCode codes.Code
	wait       bool
}

func (s *readRPC) GetOrder(ctx context.Context, request *orderpb.GetOrderRequest) (*orderpb.OrderResponse, error) {
	s.calls.Add(1)
	md, _ := metadata.FromIncomingContext(ctx)
	if len(md.Get("x-order-service-key")) != 1 || md.Get("x-order-service-key")[0] != grpcKey {
		return nil, status.Error(codes.Unauthenticated, "sensitive internal credentials diagnostic")
	}
	if s.wait {
		<-ctx.Done()
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	if s.resultCode != codes.OK {
		return nil, status.Error(s.resultCode, "sensitive internal dependency diagnostic")
	}
	s.seen <- request
	return &orderpb.OrderResponse{OrderId: request.OrderId, Status: orderpb.OrderStatus_CONFIRMED, Currency: "INR", TotalAmountMinor: 10005, Items: []*orderpb.OrderItem{{ItemNameSnapshot: "Stored dish", ItemPriceSnapshotMinor: 5005, LineTotalMinor: 10010, Quantity: 2}}}, nil
}

func rpcReader(t *testing.T, service *readRPC, key string) *grpcclient.OrderServiceClient {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	orderpb.RegisterOrderServiceServer(server, service)
	go server.Serve(listener)
	t.Cleanup(server.Stop)
	cfg := &config.Config{}
	cfg.App.Env = "test"
	cfg.GRPC.OrderAddr = listener.Addr().String()
	cfg.GRPC.OrderServiceKey = key
	cfg.GRPC.OrderTimeoutMS = 100
	reader, err := grpcclient.NewOrderServiceClient(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reader.Close() })
	return reader
}

func readHTTP(reader OrderReader, uid, role, id string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constants.AuthContextUserIDKey, uid)
		c.Set(constants.AuthContextRoleKey, role)
	})
	router.GET("/orders/:orderId", NewGRPCReadHandler(reader))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orders/"+id+"?requester_user_id=attacker&requester_role=admin", nil))
	return rec
}

func TestHTTPActuallyInvokesTypedOrderRPC(t *testing.T) {
	service := &readRPC{seen: make(chan *orderpb.GetOrderRequest, 1)}
	reader := rpcReader(t, service, grpcKey)
	rec := readHTTP(reader, grpcUser, constants.RoleClient, grpcOrder)
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	request := <-service.seen
	if request.RequesterUserId != grpcUser || request.RequesterRole != "client" || request.OrderId != grpcOrder {
		t.Fatal("HTTP supplied identity was delegated")
	}
	var body struct {
		Data struct {
			Total    int64  `json:"total_amount_minor"`
			Currency string `json:"currency"`
			Status   string `json:"status"`
			Items    []struct {
				Price int64 `json:"item_price_snapshot_minor"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Total != 10005 || body.Data.Currency != "INR" || body.Data.Status != "confirmed" || body.Data.Items[0].Price != 5005 {
		t.Fatal("exact RPC snapshot fields not exposed")
	}
}

func TestHTTPOrderReadRejectsBeforeRPC(t *testing.T) {
	service := &readRPC{seen: make(chan *orderpb.GetOrderRequest, 1)}
	reader := rpcReader(t, service, grpcKey)
	for _, tc := range []struct {
		uid, role, id string
		want          int
	}{
		{"", "client", grpcOrder, 401}, {grpcUser, "driver", grpcOrder, 403},
		{grpcUser, "client", "invalid", 400},
	} {
		if rec := readHTTP(reader, tc.uid, tc.role, tc.id); rec.Code != tc.want {
			t.Fatalf("status %d want %d", rec.Code, tc.want)
		}
	}
	if service.calls.Load() != 0 {
		t.Fatal("invalid request reached gRPC")
	}
}

func TestHTTPOrderRPCErrorMapping(t *testing.T) {
	for code, want := range map[codes.Code]int{codes.NotFound: 404, codes.InvalidArgument: 400, codes.PermissionDenied: 403, codes.DeadlineExceeded: 504, codes.Unavailable: 503, codes.Canceled: 503, codes.Unauthenticated: 502, codes.Internal: 502, codes.Unimplemented: 502} {
		t.Run(code.String(), func(t *testing.T) {
			reader := rpcReader(t, &readRPC{resultCode: code}, grpcKey)
			rec := readHTTP(reader, grpcUser, "client", grpcOrder)
			if rec.Code != want {
				t.Fatalf("status %d want %d", rec.Code, want)
			}
			if strings.Contains(rec.Body.String(), "sensitive") {
				t.Fatal("dependency diagnostics leaked")
			}
		})
	}
}

func TestHTTPOrderRPCHasBoundedDeadlineAndNoFallback(t *testing.T) {
	service := &readRPC{wait: true}
	reader := rpcReader(t, service, grpcKey)
	start := time.Now()
	rec := readHTTP(reader, grpcUser, "client", grpcOrder)
	if rec.Code != 504 || time.Since(start) > time.Second {
		t.Fatalf("deadline not enforced: %d", rec.Code)
	}
	if service.calls.Load() != 1 {
		t.Fatal("RPC retry/fallback occurred")
	}
}

func TestOrderReadRouteIsExplicitlyEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, enabled := range []bool{false, true} {
		cfg := &config.Config{}
		cfg.GRPC.OrderRequired = enabled
		router := gin.New()
		RegisterRoutes(router.Group("/api/v1"), &app.Container{Config: cfg, OrderClient: &grpcclient.OrderServiceClient{}})
		found := false
		for _, route := range router.Routes() {
			if route.Method == "GET" && route.Path == "/api/v1/orders/:orderId" {
				found = true
			}
		}
		if found != enabled {
			t.Fatal("order read route enablement mismatch")
		}
	}
}
