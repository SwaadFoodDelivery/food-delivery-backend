package api

import (
	"database/sql"
	"net/http"
	"strings"

	"food-delivery-backend/internal/constants"
	"food-delivery-backend/internal/services/restaurant/business"
	"food-delivery-backend/internal/services/restaurant/models"
	"food-delivery-backend/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ownerHandler struct{ svc business.OwnerService }

type itemMutationRequest struct {
	Name               string   `json:"name" binding:"required"`
	Description        string   `json:"description"`
	Price              int64    `json:"price_minor" binding:"required"`
	ImageURL           string   `json:"image_url"`
	IsVeg              bool     `json:"is_veg"`
	IsAvailable        bool     `json:"is_available"`
	PreparationTimeMin int      `json:"preparation_time_min"`
	Tags               []string `json:"tags"`
}

func newOwnerHandler(svc business.OwnerService) *ownerHandler { return &ownerHandler{svc: svc} }

func (h *ownerHandler) CreateItem(c *gin.Context) {
	actorID, restaurantID, categoryID, ok := ownerIDs(c, true)
	if !ok {
		return
	}
	var req itemMutationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_MENU_ITEM", "invalid menu item", []string{})
		return
	}
	out, err := h.svc.CreateItem(c.Request.Context(), actorID, restaurantID, categoryID, req.model())
	if err != nil {
		ownerError(c, err)
		return
	}
	response.Success(c, http.StatusCreated, out)
}

func (h *ownerHandler) UpdateItem(c *gin.Context) {
	actorID, restaurantID, itemID, ok := ownerIDs(c, false)
	if !ok {
		return
	}
	var req itemMutationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_MENU_ITEM", "invalid menu item", []string{})
		return
	}
	out, err := h.svc.UpdateItem(c.Request.Context(), actorID, restaurantID, itemID, req.model())
	if err != nil {
		ownerError(c, err)
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *ownerHandler) DeleteItem(c *gin.Context) {
	actorID, restaurantID, itemID, ok := ownerIDs(c, false)
	if !ok {
		return
	}
	if err := h.svc.DeleteItem(c.Request.Context(), actorID, restaurantID, itemID); err != nil {
		ownerError(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}

func (r itemMutationRequest) model() models.Item {
	return models.Item{Name: strings.TrimSpace(r.Name), Description: strings.TrimSpace(r.Description), Price: r.Price, ImageURL: strings.TrimSpace(r.ImageURL), IsVeg: r.IsVeg, IsAvailable: r.IsAvailable, PreparationTimeMin: r.PreparationTimeMin, Tags: r.Tags}
}

func ownerIDs(c *gin.Context, needsCategory bool) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	actorRaw, ok := c.Get(constants.AuthContextUserIDKey)
	actorID, err := uuid.Parse(strings.TrimSpace(valueString(actorRaw)))
	if !ok || err != nil {
		response.Error(c, http.StatusUnauthorized, "INVALID_TOKEN", "invalid user identity", []string{})
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	restaurantID, err := uuid.Parse(c.Param("restaurantId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_RESTAURANT_ID", "restaurantId must be a UUID", []string{})
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	if !needsCategory {
		itemID, err := uuid.Parse(c.Param("itemId"))
		if err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_ITEM_ID", "itemId must be a UUID", []string{})
			return uuid.Nil, uuid.Nil, uuid.Nil, false
		}
		return actorID, restaurantID, itemID, true
	}
	categoryID, err := uuid.Parse(c.Param("categoryId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_CATEGORY_ID", "categoryId must be a UUID", []string{})
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return actorID, restaurantID, categoryID, true
}

func valueString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func ownerError(c *gin.Context, err error) {
	if err == sql.ErrNoRows || strings.Contains(err.Error(), "not found") {
		response.Error(c, http.StatusNotFound, "MENU_ITEM_NOT_FOUND", "menu item not found", []string{})
		return
	}
	response.Error(c, http.StatusBadRequest, "INVALID_MENU_ITEM", err.Error(), []string{})
}
