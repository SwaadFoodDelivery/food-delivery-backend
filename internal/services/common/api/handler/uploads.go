package handler

import (
	"net/http"
	"path"
	"strings"
	"time"

	"food-delivery-backend/internal/app"
	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/internal/services/common/storage"

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
	Key         string `json:"key"`
	Entity      string `json:"entity"`
	Onboarding  string `json:"onboarding_id"`
	Document    string `json:"document_type"`
	ContentType string `json:"content_type"`
	Extension   string `json:"extension"`
}

func (h *UploadsHandler) PresignURL(c *gin.Context) {
	if h.storage == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error_code": "STORAGE_NOT_CONFIGURED", "message": "storage provider is not configured", "details": []string{}})
		return
	}

	var req presignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error_code": "VALIDATION_ERROR", "message": "invalid request body", "details": []string{}})
		return
	}

	userID, _ := c.Get(middleware.ContextUserIDKey)
	ownerPrefix := "users/" + strings.TrimSpace(toString(userID)) + "/"
	if ownerPrefix == "users//" {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "error_code": "UNAUTHORIZED", "message": "user context missing", "details": []string{}})
		return
	}

	bucket := strings.TrimSpace(req.Bucket)
	if bucket == "" {
		bucket = strings.TrimSpace(h.deps.Config.S3.Bucket)
	}
	if bucket == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error_code": "VALIDATION_ERROR", "message": "bucket is required", "details": []string{}})
		return
	}

	contentType := strings.TrimSpace(req.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	key := strings.TrimSpace(req.Key)
	if key == "" {
		if strings.TrimSpace(req.Entity) != "onboarding" || strings.TrimSpace(req.Onboarding) == "" || strings.TrimSpace(req.Document) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error_code": "VALIDATION_ERROR", "message": "either key or (entity=onboarding, onboarding_id, document_type) is required", "details": []string{}})
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
		c.JSON(http.StatusForbidden, gin.H{"status": "error", "error_code": "INVALID_OBJECT_KEY", "message": "object key must be owner scoped", "details": []string{}})
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
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error_code": "PRESIGN_FAILED", "message": err.Error(), "details": []string{}})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"bucket":     bucket,
			"key":        key,
			"upload_url": out.URL,
			"method":     out.Method,
			"headers":    out.Headers,
			"expires_at": out.ExpiresAt.Format(time.RFC3339),
		},
	})
}

func cleanObjectKey(key string) string {
	clean := path.Clean("/" + key)
	return strings.TrimPrefix(clean, "/")
}

func toString(v any) string {
	s, _ := v.(string)
	return s
}
