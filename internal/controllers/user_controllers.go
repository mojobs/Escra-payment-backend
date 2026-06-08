package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/services"
)

type UserController struct {
	userService *services.UserService
}

func NewUserController(userService *services.UserService) *UserController {
	return &UserController{userService: userService}
}

func (ctrl *UserController) GetProfile(c *gin.Context) {
	userID := c.GetString("user_id")

	profile, err := ctrl.userService.GetProfile(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "user_not_found",
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (ctrl *UserController) UpdateMerchantDetails(c *gin.Context) {
	userID := c.GetString("user_id")

	var req models.UpdateMerchantDetailsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	if err := ctrl.userService.UpdateMerchantDetails(userID, &req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "merchant_profile_update_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: "Merchant profile details updated successfully",
	})
}
