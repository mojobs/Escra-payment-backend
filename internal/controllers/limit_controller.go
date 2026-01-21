package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/services"
)

type LimitController struct {
	limitService *services.TransactionLimitService
}

func NewLimitController(limitService *services.TransactionLimitService) *LimitController {
	return &LimitController{
		limitService: limitService,
	}
}

func (ctrl *LimitController) GetLimits(c *gin.Context) {
	userIDStr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_user_id",
			Message: "Invalid user ID format",
		})
		return
	}

	stats, err := ctrl.limitService.GetUsageStats(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error : "fetch_failed",
			Message : err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, stats)
}