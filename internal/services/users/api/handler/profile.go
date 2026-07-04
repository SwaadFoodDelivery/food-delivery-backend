package handler

import (
	"net/http"
	"strconv"

	"food-delivery-backend/internal/constants"
	apperrors "food-delivery-backend/internal/errors"
	"food-delivery-backend/internal/middleware"
	"food-delivery-backend/internal/services/users/business"
	"food-delivery-backend/internal/services/users/models"
	"food-delivery-backend/internal/services/users/validations"
	"food-delivery-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type ProfileHandler struct {
	svc business.ProfileService
}

func NewProfileHandler(svc business.ProfileService) *ProfileHandler {
	return &ProfileHandler{svc: svc}
}

func (h *ProfileHandler) GetProfile(c *gin.Context) {
	userID, _ := c.Get(constants.AuthContextUserIDKey)

	out, svcErr := h.svc.GetProfile(c.Request.Context(), models.GetProfileInput{
		UserID: toString(userID),
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *ProfileHandler) UpdateProfile(c *gin.Context) {
	req, ok := middleware.GetValidatedBody[models.UpdateProfileRequest](c)
	if !ok {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "invalid request body", []string{})
		return
	}

	userID, _ := c.Get(constants.AuthContextUserIDKey)
	role, _ := c.Get(constants.AuthContextRoleKey)

	out, svcErr := h.svc.UpdateProfile(c.Request.Context(), models.UpdateProfileInput{
		UserID:      toString(userID),
		Role:        toString(role),
		Name:        req.Name,
		Email:       req.Email,
		DateOfBirth: req.DateOfBirth,
		Gender:      req.Gender,
		IsAvailable: req.IsAvailable,
		CurrentCity: req.CurrentCity,
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *ProfileHandler) GetAddresses(c *gin.Context) {
	userID, _ := c.Get(constants.AuthContextUserIDKey)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(constants.DefaultAddressListLimit)))

	out, svcErr := h.svc.GetAddresses(c.Request.Context(), models.GetAddressesInput{
		UserID: toString(userID),
		Page:   page,
		Limit:  limit,
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *ProfileHandler) CreateAddress(c *gin.Context) {
	req, ok := middleware.GetValidatedBody[models.CreateAddressRequest](c)
	if !ok {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "invalid request body", []string{})
		return
	}

	userID, _ := c.Get(constants.AuthContextUserIDKey)
	role, _ := c.Get(constants.AuthContextRoleKey)

	out, svcErr := h.svc.CreateAddress(c.Request.Context(), models.CreateAddressInput{
		UserID:       toString(userID),
		ActorRole:    toString(role),
		Label:        req.Label,
		Line1:        req.Line1,
		Line2:        req.Line2,
		Area:         req.Area,
		City:         req.City,
		State:        req.State,
		Pincode:      req.Pincode,
		Latitude:     req.Latitude,
		Longitude:    req.Longitude,
		ContactName:  req.ContactName,
		ContactPhone: req.ContactPhone,
		IsDefault:    req.IsDefault,
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	response.Success(c, http.StatusCreated, out)
}

func (h *ProfileHandler) UpdateAddress(c *gin.Context) {
	req, ok := middleware.GetValidatedBody[models.UpdateAddressRequest](c)
	if !ok {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, "invalid request body", []string{})
		return
	}

	userID, _ := c.Get(constants.AuthContextUserIDKey)
	role, _ := c.Get(constants.AuthContextRoleKey)
	addressID, errMsg := validations.ValidateAddressID(c.Param("addressId"))
	if errMsg != "" {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, errMsg, []string{})
		return
	}

	out, svcErr := h.svc.UpdateAddress(c.Request.Context(), models.UpdateAddressInput{
		UserID:       toString(userID),
		ActorRole:    toString(role),
		AddressID:    addressID,
		Label:        req.Label,
		Line1:        req.Line1,
		Line2:        req.Line2,
		Area:         req.Area,
		City:         req.City,
		State:        req.State,
		Pincode:      req.Pincode,
		Latitude:     req.Latitude,
		Longitude:    req.Longitude,
		ContactName:  req.ContactName,
		ContactPhone: req.ContactPhone,
		IsDefault:    req.IsDefault,
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	response.Success(c, http.StatusOK, out)
}

func (h *ProfileHandler) DeleteAddress(c *gin.Context) {
	userID, _ := c.Get(constants.AuthContextUserIDKey)
	role, _ := c.Get(constants.AuthContextRoleKey)
	addressID, errMsg := validations.ValidateAddressID(c.Param("addressId"))
	if errMsg != "" {
		response.Error(c, http.StatusBadRequest, apperrors.CodeValidation, errMsg, []string{})
		return
	}

	svcErr := h.svc.DeleteAddress(c.Request.Context(), models.DeleteAddressInput{
		UserID:    toString(userID),
		ActorRole: toString(role),
		AddressID: addressID,
	})
	if svcErr != nil {
		writeServiceError(c, svcErr)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}
