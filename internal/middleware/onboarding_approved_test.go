package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/services/users/models"
	"food-delivery-backend/internal/services/users/repository/repository"
	"food-delivery-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

const approvedOnboardingUserID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"

type approvedOnboardingRepository struct {
	repository.Repository
	user    *models.UserRow
	err     error
	calls   int
	userID  string
	context context.Context
}

func (r *approvedOnboardingRepository) FindUserByID(ctx context.Context, userID string) (*models.UserRow, error) {
	r.calls++
	r.userID = userID
	r.context = ctx
	return r.user, r.err
}

func TestRequireApprovedOnboarding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := func(role, status string, complete bool) *models.UserRow {
		return &models.UserRow{UserID: approvedOnboardingUserID, Role: role, AccountStatus: status, OnboardingComplete: complete}
	}
	owner := constants.RoleRestaurantOwner
	driver := constants.RoleDriver
	active := constants.AccountStatusActive
	approved := user(owner, active, true)

	tests := []struct {
		name       string
		userID     any
		role       any
		user       *models.UserRow
		err        error
		nilRepo    bool
		wantStatus int
		wantCode   string
		wantCalls  int
	}{
		{name: "unauthenticated", user: approved, wantStatus: http.StatusUnauthorized, wantCode: apperrors.CodeUnauthorized},
		{name: "missing user ID", role: owner, user: approved, wantStatus: http.StatusUnauthorized, wantCode: apperrors.CodeUnauthorized},
		{name: "missing role", userID: approvedOnboardingUserID, user: approved, wantStatus: http.StatusUnauthorized, wantCode: apperrors.CodeUnauthorized},
		{name: "empty user ID", userID: "", role: owner, user: approved, wantStatus: http.StatusUnauthorized, wantCode: apperrors.CodeUnauthorized},
		{name: "blank user ID", userID: " \t", role: owner, user: approved, wantStatus: http.StatusUnauthorized, wantCode: apperrors.CodeUnauthorized},
		{name: "non-string user ID", userID: 123, role: owner, user: approved, wantStatus: http.StatusUnauthorized, wantCode: apperrors.CodeUnauthorized},
		{name: "malformed user ID", userID: "invalid-uuid", role: owner, user: approved, wantStatus: http.StatusUnauthorized, wantCode: apperrors.CodeUnauthorized},
		{name: "empty role", userID: approvedOnboardingUserID, role: "", user: approved, wantStatus: http.StatusUnauthorized, wantCode: apperrors.CodeUnauthorized},
		{name: "blank role", userID: approvedOnboardingUserID, role: " \t", user: approved, wantStatus: http.StatusUnauthorized, wantCode: apperrors.CodeUnauthorized},
		{name: "non-string role", userID: approvedOnboardingUserID, role: 123, user: approved, wantStatus: http.StatusUnauthorized, wantCode: apperrors.CodeUnauthorized},
		{name: "unapproved owner", userID: approvedOnboardingUserID, role: owner, user: user(owner, active, false), wantStatus: http.StatusForbidden, wantCode: apperrors.CodeForbidden, wantCalls: 1},
		{name: "unapproved driver", userID: approvedOnboardingUserID, role: driver, user: user(driver, active, false), wantStatus: http.StatusForbidden, wantCode: apperrors.CodeForbidden, wantCalls: 1},
		{name: "approved seeded owner", userID: approvedOnboardingUserID, role: owner, user: approved, wantStatus: http.StatusNoContent, wantCalls: 1},
		{name: "approved seeded driver", userID: approvedOnboardingUserID, role: driver, user: user(driver, active, true), wantStatus: http.StatusNoContent, wantCalls: 1},
		{name: "suspended owner", userID: approvedOnboardingUserID, role: owner, user: user(owner, constants.AccountStatusSuspended, true), wantStatus: http.StatusForbidden, wantCode: apperrors.CodeAccountNotActive, wantCalls: 1},
		{name: "suspended driver", userID: approvedOnboardingUserID, role: driver, user: user(driver, constants.AccountStatusSuspended, true), wantStatus: http.StatusForbidden, wantCode: apperrors.CodeAccountNotActive, wantCalls: 1},
		{name: "unknown account status", userID: approvedOnboardingUserID, role: owner, user: user(owner, "", true), wantStatus: http.StatusForbidden, wantCode: apperrors.CodeAccountNotActive, wantCalls: 1},
		{name: "role mismatch", userID: approvedOnboardingUserID, role: owner, user: user(driver, active, true), wantStatus: http.StatusForbidden, wantCode: apperrors.CodeForbidden, wantCalls: 1},
		{name: "missing database role", userID: approvedOnboardingUserID, role: owner, user: user("", active, true), wantStatus: http.StatusForbidden, wantCode: apperrors.CodeForbidden, wantCalls: 1},
		{name: "user not found", userID: approvedOnboardingUserID, role: owner, err: fmt.Errorf("lookup: %w", apperrors.ErrNotFound), wantStatus: http.StatusUnauthorized, wantCode: apperrors.CodeUserNotFound, wantCalls: 1},
		{name: "nil user", userID: approvedOnboardingUserID, role: owner, wantStatus: http.StatusUnauthorized, wantCode: apperrors.CodeUserNotFound, wantCalls: 1},
		{name: "database failure", userID: approvedOnboardingUserID, role: owner, err: errors.New("private database error"), wantStatus: http.StatusInternalServerError, wantCode: apperrors.CodeInternal, wantCalls: 1},
		{name: "database failure with user", userID: approvedOnboardingUserID, role: owner, user: approved, err: errors.New("private database error"), wantStatus: http.StatusInternalServerError, wantCode: apperrors.CodeInternal, wantCalls: 1},
		{name: "repository not configured", userID: approvedOnboardingUserID, role: owner, nilRepo: true, wantStatus: http.StatusInternalServerError, wantCode: apperrors.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &approvedOnboardingRepository{user: tt.user, err: tt.err}
			var guardRepo repository.Repository = repo
			if tt.nilRepo {
				guardRepo = nil
			}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				if tt.userID != nil {
					c.Set(constants.AuthContextUserIDKey, tt.userID)
				}
				if tt.role != nil {
					c.Set(constants.AuthContextRoleKey, tt.role)
				}
			})
			handlerCalled := false
			router.GET("/protected", RequireApprovedOnboarding(guardRepo), func(c *gin.Context) {
				handlerCalled = true
				c.Status(http.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if handlerCalled != (tt.wantStatus == http.StatusNoContent) {
				t.Fatalf("handler called = %v for status %d", handlerCalled, tt.wantStatus)
			}
			if repo.calls != tt.wantCalls {
				t.Fatalf("database lookups = %d, want %d", repo.calls, tt.wantCalls)
			}
			if repo.calls > 0 && (repo.userID != approvedOnboardingUserID || repo.context != req.Context()) {
				t.Fatal("lookup did not receive the authenticated user ID and request context")
			}
			if tt.wantCode != "" {
				var body response.APIResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.ErrorCode != tt.wantCode || body.Status != constants.ResponseStatusError {
					t.Fatalf("unexpected error response: %+v", body)
				}
				if strings.Contains(rec.Body.String(), "private database error") {
					t.Fatal("response exposed the database error")
				}
			}
		})
	}
}

func TestRequireApprovedOnboardingReloadsUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := &models.UserRow{
		UserID: approvedOnboardingUserID, Role: constants.RoleDriver,
		AccountStatus: constants.AccountStatusActive,
	}
	repo := &approvedOnboardingRepository{user: user}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constants.AuthContextUserIDKey, approvedOnboardingUserID)
		c.Set(constants.AuthContextRoleKey, constants.RoleDriver)
	})
	handlerCalls := 0
	router.GET("/protected", RequireApprovedOnboarding(repo), func(c *gin.Context) {
		handlerCalls++
		c.Status(http.StatusNoContent)
	})
	steps := []struct {
		name       string
		complete   bool
		status     string
		role       string
		wantStatus int
	}{
		{"pending", false, constants.AccountStatusActive, constants.RoleDriver, http.StatusForbidden},
		{"approved", true, constants.AccountStatusActive, constants.RoleDriver, http.StatusNoContent},
		{"approval revoked", false, constants.AccountStatusActive, constants.RoleDriver, http.StatusForbidden},
		{"suspended", true, constants.AccountStatusSuspended, constants.RoleDriver, http.StatusForbidden},
		{"role changed", true, constants.AccountStatusActive, constants.RoleRestaurantOwner, http.StatusForbidden},
	}
	for i, step := range steps {
		user.OnboardingComplete, user.AccountStatus, user.Role = step.complete, step.status, step.role
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/protected", nil))
		if rec.Code != step.wantStatus || repo.calls != i+1 {
			t.Fatalf("%s: status = %d, want %d; lookups = %d, want %d", step.name, rec.Code, step.wantStatus, repo.calls, i+1)
		}
	}
	if handlerCalls != 1 {
		t.Fatalf("handler calls = %d, want 1", handlerCalls)
	}
}
