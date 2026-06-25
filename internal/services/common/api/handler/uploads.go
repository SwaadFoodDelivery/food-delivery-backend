package handler

import (
	"net/http"
	"path"
	"strings"
	"time"

	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/services/common/storage"
	"food-delivery-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type UploadsHandler struct {
	storage storage.Provider
	deps    *app.Container
}

func NewUploadsHandler(deps *app.Container) *UploadsHandler {
	return &UploadsHandler{storage: deps.StorageProvider, deps: deps}
}

type presignRequest struct {
	Bucket      string `json:"bucket"`
	Purpose     string `json:"purpose"`
	Key         string `json:"key"`
	Entity      string `json:"entity"`
	Onboarding  string `json:"onboarding_id"`
	Document    string `json:"document_type"`
	ContentType string `json:"content_type"`
	Extension   string `json:"extension"`
}

func (h *UploadsHandler) PresignURL(c *gin.Context) {
	if h.storage == nil {
		response.Error(c, http.StatusInternalServerError, apperrors.CodeOnboardingStorageNotConfigured, "storage provider is not configured", []string{})
		return
	}

	var req presignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "invalid request body", []string{})
		return
	}

	userID, _ := c.Get(constants.AuthContextUserIDKey)
	ownerPrefix := "users/" + strings.TrimSpace(toString(userID)) + "/"
	if ownerPrefix == "users//" {
		response.Error(c, http.StatusUnauthorized, apperrors.CodeUnauthorized, "user context missing", []string{})
		return
	}

	bucket := strings.TrimSpace(req.Bucket)
	if bucket == "" {
		bucket = h.deps.Config.S3BucketFor(bucketPurpose(req))
	}
	if bucket == "" {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "bucket is required", []string{})
		return
	}

	contentType := strings.TrimSpace(req.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	key := strings.TrimSpace(req.Key)
	if key == "" {
		if strings.TrimSpace(req.Entity) != "onboarding" || strings.TrimSpace(req.Onboarding) == "" || strings.TrimSpace(req.Document) == "" {
			response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "either key or (entity=onboarding, onboarding_id, document_type) is required", []string{})
			return
		}
		ext := strings.Trim(strings.TrimSpace(req.Extension), ".")
		fileName := strings.TrimSpace(req.Document)
		if ext != "" {
			fileName += "." + ext
		}
		key = ownerPrefix + "onboarding/" + strings.TrimSpace(req.Onboarding) + "/" + fileName
	}

	if !strings.HasPrefix(key, ownerPrefix) {
		response.Error(c, http.StatusForbidden, apperrors.CodeOnboardingInvalidObjectKey, "object key must be owner scoped", []string{})
		return
	}

	key = cleanObjectKey(key)
	out, err := h.storage.PresignPut(c.Request.Context(), storage.PresignPutInput{
		Bucket:      bucket,
		Key:         key,
		ContentType: contentType,
		ExpiresIn:   time.Duration(h.deps.Config.S3.PresignTTLSeconds) * time.Second,
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, apperrors.CodeOnboardingPresignFailed, err.Error(), []string{})
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"bucket":     bucket,
		"key":        key,
		"upload_url": out.URL,
		"method":     out.Method,
		"headers":    out.Headers,
		"expires_at": out.ExpiresAt.Format(time.RFC3339),
	})
}

func bucketPurpose(req presignRequest) string {
	purpose := strings.ToLower(strings.TrimSpace(req.Purpose))
	if purpose != "" {
		return purpose
	}
	if strings.TrimSpace(req.Entity) == constants.S3BucketPurposeOnboarding {
		return constants.S3BucketPurposeOnboarding
	}
	return constants.S3BucketPurposeDefault
}

func cleanObjectKey(key string) string {
	clean := path.Clean("/" + key)
	return strings.TrimPrefix(clean, "/")
}

func toString(v any) string {
	s, _ := v.(string)
	return s
}
