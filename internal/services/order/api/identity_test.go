package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"food-delivery-backend/internal/constants"
	ordermodels "food-delivery-backend/internal/services/order/models"
	"food-delivery-backend/internal/services/order/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type identityService struct {
	fakeService
	err error
}

func (s identityService) List(context.Context, uuid.UUID, int) ([]ordermodels.HistoryItem, error) {
	return nil, s.err
}
func (s identityService) History(context.Context, uuid.UUID, uuid.UUID) (ordermodels.History, error) {
	return ordermodels.History{}, s.err
}
func (s identityService) Cancel(context.Context, uuid.UUID, uuid.UUID) (ordermodels.HistoryItem, error) {
	return ordermodels.HistoryItem{}, s.err
}

func TestOrderIdentityHTTPErrorMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, method := range []string{http.MethodGet, http.MethodPatch} {
		for _, tc := range []struct {
			name   string
			err    error
			status int
			code   string
		}{
			{"ambiguous", fmt.Errorf("lookup: %w", repository.ErrOrderAmbiguous), 409, "ORDER_ID_AMBIGUOUS"},
			{"not_found", repository.ErrOrderNotFound, 404, "ORDER_NOT_FOUND"},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				r := gin.New()
				r.Use(func(c *gin.Context) { c.Set(constants.AuthContextUserIDKey, uuid.NewString()); c.Next() })
				h := NewHandler(identityService{err: tc.err})
				if method == http.MethodGet {
					r.GET("/orders/:orderId/history", h.History)
				} else {
					r.PATCH("/orders/:orderId/history", h.Cancel)
				}
				response := httptest.NewRecorder()
				r.ServeHTTP(response, httptest.NewRequest(method, "/orders/"+uuid.NewString()+"/history", nil))
				if response.Code != tc.status {
					t.Fatalf("HTTP %d, want %d: %s", response.Code, tc.status, response.Body.String())
				}
				var body struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Error.Code != tc.code {
					t.Fatalf("error code %q, want %q", body.Error.Code, tc.code)
				}
			})
		}
	}
}
