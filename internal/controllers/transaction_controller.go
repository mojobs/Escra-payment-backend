package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/services"
)

type TransactionController struct {
	transactionService *services.TransactionService
}

func NewTransactionController(transactionService *services.TransactionService) *TransactionController {
	return &TransactionController{transactionService: transactionService}
}

// Transfers handles money transfer requests

func (ctrl *TransactionController) Transfer(c *gin.Context) {
	var req models.TransferRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	userIDstr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDstr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_user_id",
			Message: "Invalid user ID format",
		})
		return
	}

	response, err := ctrl.transactionService.Transfer(userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "transfer-failed",
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, response)
}

func (ctrl *TransactionController) GetHistory(c *gin.Context) {
	userIDstr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDstr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_user_id",
			Message: "Invalid user ID format",
		})
		return
	}

	//Get limit from query params (default 50)
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100 // Max Limit
	}

	history, err := ctrl.transactionService.GetTransactionHistory(userID, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "fetch_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"transactions": history,
		"count":        len(history),
	})
}

func (ctrl *TransactionController) GetByReference(c *gin.Context) {
	reference := c.Param("reference")
	userIDstr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDstr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_user_id",
			Message: "Invalid user ID format",
		})
		return
	}

	transaction, err := ctrl.transactionService.GetTransactionByReference(userID, reference)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "not_found",
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, transaction)

}
