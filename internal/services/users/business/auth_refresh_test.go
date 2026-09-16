package business_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/services/users/business"
	"food-delivery-backend/internal/services/users/models"
	redisstore "food-delivery-backend/internal/services/users/repository/redis"
	"food-delivery-backend/internal/services/users/repository/repository"
	"food-delivery-backend/pkg/config"
	"food-delivery-backend/pkg/utils"

	"github.com/rs/zerolog"
)

const refreshTestSecret = "test-jwt-secret"

type refreshFakeRepository struct {
	repository.Repository
	session    *redisstore.SessionRecord
	getErr     error
	touchErr   error
	getCalls   int
	touchCalls int
	touchedTTL time.Duration
}

func (r *refreshFakeRepository) GetSession(ctx context.Context, sessionID string) (*redisstore.SessionRecord, error) {
	r.getCalls++
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.session, nil
}

func (r *refreshFakeRepository) TouchSession(ctx context.Context, sessionID string, ttl time.Duration) error {
	r.touchCalls++
	r.touchedTTL = ttl
	if r.touchErr != nil {
		return r.touchErr
	}
	return nil
}

func refreshTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.JWT.Secret = refreshTestSecret
	return cfg
}

func validRefreshToken(t *testing.T, userID, sessionID string, ttl time.Duration) string {
	t.Helper()
	token, err := utils.CreateRefreshToken(refreshTestSecret, userID, sessionID, ttl)
	if err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}
	return token
}

func TestServiceRefresh(t *testing.T) {
	const userID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	const sessionID = "session-refresh-1"
	activeSession := &redisstore.SessionRecord{UserID: userID, Role: "client", IsActive: true}

	tests := []struct {
		name         string
		token        string
		repo         *refreshFakeRepository
		wantStatus   int
		wantCode     string
		wantGetCalls int
	}{
		{
			name:       "missing token",
			token:      "",
			repo:       &refreshFakeRepository{},
			wantStatus: http.StatusUnauthorized,
			wantCode:   apperrors.CodeInvalidToken,
		},
		{
			name:       "malformed token",
			token:      "not-a-jwt",
			repo:       &refreshFakeRepository{},
			wantStatus: http.StatusUnauthorized,
			wantCode:   apperrors.CodeInvalidToken,
		},
		{
			name:       "expired token",
			token:      validRefreshToken(t, userID, sessionID, -time.Minute),
			repo:       &refreshFakeRepository{},
			wantStatus: http.StatusUnauthorized,
			wantCode:   apperrors.CodeTokenExpired,
		},
		{
			name:         "session not found",
			token:        validRefreshToken(t, userID, sessionID, time.Hour),
			repo:         &refreshFakeRepository{getErr: apperrors.ErrNotFound},
			wantStatus:   http.StatusUnauthorized,
			wantCode:     apperrors.CodeSessionNotFound,
			wantGetCalls: 1,
		},
		{
			name:         "session revoked",
			token:        validRefreshToken(t, userID, sessionID, time.Hour),
			repo:         &refreshFakeRepository{session: &redisstore.SessionRecord{UserID: userID, Role: "client", IsActive: false}},
			wantStatus:   http.StatusUnauthorized,
			wantCode:     apperrors.CodeSessionRevoked,
			wantGetCalls: 1,
		},
		{
			name:         "session belongs to a different user",
			token:        validRefreshToken(t, userID, sessionID, time.Hour),
			repo:         &refreshFakeRepository{session: &redisstore.SessionRecord{UserID: "someone-else", Role: "client", IsActive: true}},
			wantStatus:   http.StatusUnauthorized,
			wantCode:     apperrors.CodeInvalidToken,
			wantGetCalls: 1,
		},
		{
			name:         "session lookup failure",
			token:        validRefreshToken(t, userID, sessionID, time.Hour),
			repo:         &refreshFakeRepository{getErr: errors.New("redis down")},
			wantStatus:   http.StatusInternalServerError,
			wantCode:     apperrors.CodeInternal,
			wantGetCalls: 1,
		},
		{
			name:         "renewal failure",
			token:        validRefreshToken(t, userID, sessionID, time.Hour),
			repo:         &refreshFakeRepository{session: activeSession, touchErr: errors.New("redis down")},
			wantStatus:   http.StatusInternalServerError,
			wantCode:     apperrors.CodeInternal,
			wantGetCalls: 1,
		},
		{
			name:         "success",
			token:        validRefreshToken(t, userID, sessionID, time.Hour),
			repo:         &refreshFakeRepository{session: activeSession},
			wantGetCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := business.NewService(tt.repo, refreshTestConfig(), zerolog.Nop(), nil, nil, nil)
			out, svcErr := svc.Refresh(context.Background(), models.RefreshInput{RefreshToken: tt.token})

			if tt.wantCode != "" {
				if svcErr == nil {
					t.Fatal("expected an error, got none")
				}
				if svcErr.StatusCode != tt.wantStatus || svcErr.Code != tt.wantCode {
					t.Fatalf("error = %d/%s, want %d/%s", svcErr.StatusCode, svcErr.Code, tt.wantStatus, tt.wantCode)
				}
			} else {
				if svcErr != nil {
					t.Fatalf("unexpected error: %+v", svcErr)
				}
				if out == nil || out.AccessToken == "" {
					t.Fatal("expected a minted access token")
				}
				claims, err := utils.ParseAccessToken(refreshTestSecret, out.AccessToken)
				if err != nil {
					t.Fatalf("minted access token does not parse: %v", err)
				}
				if claims.UserID != userID || claims.Role != "client" || claims.ID != sessionID {
					t.Fatalf("unexpected minted claims: %+v", claims)
				}
				if out.TokenType != constants.BearerTokenType {
					t.Fatalf("token_type = %q, want %q", out.TokenType, constants.BearerTokenType)
				}
				if tt.repo.touchCalls != 1 || tt.repo.touchedTTL != constants.AuthSessionTTL {
					t.Fatalf("session was not renewed with the session TTL: calls=%d ttl=%v", tt.repo.touchCalls, tt.repo.touchedTTL)
				}
			}

			if tt.repo.getCalls != tt.wantGetCalls {
				t.Fatalf("GetSession calls = %d, want %d", tt.repo.getCalls, tt.wantGetCalls)
			}
		})
	}
}

// A refresh token signed for one session must never renew a different one --
// there is no field in the request that names a session, only the token's own
// claims, so this is the entire boundary against cross-session replay.
func TestServiceRefreshUsesTokenSessionNotAnyActiveSession(t *testing.T) {
	const userID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	token := validRefreshToken(t, userID, "session-a", time.Hour)
	repo := &refreshFakeRepository{session: &redisstore.SessionRecord{UserID: userID, Role: "client", IsActive: true}}
	svc := business.NewService(repo, refreshTestConfig(), zerolog.Nop(), nil, nil, nil)

	out, svcErr := svc.Refresh(context.Background(), models.RefreshInput{RefreshToken: token})
	if svcErr != nil {
		t.Fatalf("unexpected error: %+v", svcErr)
	}
	claims, err := utils.ParseAccessToken(refreshTestSecret, out.AccessToken)
	if err != nil {
		t.Fatalf("minted access token does not parse: %v", err)
	}
	if claims.ID != "session-a" {
		t.Fatalf("minted access token session = %q, want %q", claims.ID, "session-a")
	}
}
