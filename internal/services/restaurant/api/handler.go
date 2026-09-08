package api

import (
	"database/sql"
	"math"
	"net/http"
	"strconv"

	"food-delivery-backend/internal/services/restaurant/business"
	"food-delivery-backend/internal/services/restaurant/repository"
	"food-delivery-backend/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ svc business.Service }

func NewHandler(svc business.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) List(c *gin.Context) {
	lat, lon, err := coordinates(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_LOCATION", err.Error(), []string{})
		return
	}
	radius, err := strconv.ParseFloat(c.DefaultQuery("radius_km", "5"), 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_RADIUS", "radius_km must be numeric", []string{})
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_LIMIT", "limit must be numeric", []string{})
		return
	}
	offset, err := repository.DecodeCursor(c.Query("cursor"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_CURSOR", "cursor is invalid", []string{})
		return
	}
	out, err := h.svc.List(c.Request.Context(), repository.SearchFilter{Latitude: lat, Longitude: lon, RadiusKM: radius, Cuisine: c.Query("cuisine"), SortBy: c.Query("sort_by"), Offset: offset, Limit: limit})
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_RESTAURANT_QUERY", err.Error(), []string{})
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("restaurantId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_RESTAURANT_ID", "restaurantId must be a UUID", []string{})
		return
	}
	var lat, lon *float64
	if c.Query("latitude") != "" || c.Query("longitude") != "" {
		a, b, coordErr := coordinates(c)
		if coordErr != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_LOCATION", coordErr.Error(), []string{})
			return
		}
		lat, lon = &a, &b
	}
	out, err := h.svc.Get(c.Request.Context(), id, lat, lon)
	if err == sql.ErrNoRows {
		response.Error(c, http.StatusNotFound, "RESTAURANT_NOT_FOUND", "restaurant not found", []string{})
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "RESTAURANT_LOOKUP_FAILED", "restaurant could not be loaded", []string{})
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *Handler) Menu(c *gin.Context) {
	id, err := uuid.Parse(c.Param("restaurantId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_RESTAURANT_ID", "restaurantId must be a UUID", []string{})
		return
	}
	out, err := h.svc.Menu(c.Request.Context(), id)
	if err == sql.ErrNoRows {
		response.Error(c, http.StatusNotFound, "MENU_NOT_FOUND", "menu not found", []string{})
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "MENU_LOOKUP_FAILED", "menu could not be loaded", []string{})
		return
	}
	response.Success(c, http.StatusOK, out)
}

func coordinates(c *gin.Context) (float64, float64, error) {
	lat, err := strconv.ParseFloat(c.Query("latitude"), 64)
	if err != nil || lat < -90 || lat > 90 {
		return 0, 0, queryError("latitude must be between -90 and 90")
	}
	if math.IsNaN(lat) || math.IsInf(lat, 0) {
		return 0, 0, queryError("latitude must be finite")
	}
	lon, err := strconv.ParseFloat(c.Query("longitude"), 64)
	if err != nil || lon < -180 || lon > 180 {
		return 0, 0, queryError("longitude must be between -180 and 180")
	}
	if math.IsNaN(lon) || math.IsInf(lon, 0) {
		return 0, 0, queryError("longitude must be finite")
	}
	return lat, lon, nil
}

type queryError string

func (e queryError) Error() string { return string(e) }
