package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/pkg/config"
	"github.com/gin-gonic/gin"
)

func TestDocumentConfirmationRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	public := r.Group("/api/v1")
	protected := r.Group("/api/v1", middleware.JWTAuthMiddleware(&config.Config{}, nil))
	RegisterOnboardingRoutes(public, protected, &app.Container{}, nil, nil)
	for _, bearer := range []string{"", "Bearer forged-token"} {
		t.Run(bearer, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/documents/uploaded", strings.NewReader(`{"s3_key":"users/victim/onboarding/application/license"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", bearer)
			res := httptest.NewRecorder()
			r.ServeHTTP(res, req)
			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
			}
		})
	}
}
