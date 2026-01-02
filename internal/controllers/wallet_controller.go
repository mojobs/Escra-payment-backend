package controllers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/services"
)

type WalletController struct {
    walletService *services.WalletService
}

func NewWalletController(walletService *services.WalletService) *WalletController {
    return &WalletController{walletService: walletService}
}

func (ctrl *WalletController) GetBalance(c *gin.Context) {
    userIDStr := c.GetString("user_id")
    userID, err := uuid.Parse(userIDStr)
    if err != nil {
        c.JSON(http.StatusBadRequest, models.ErrorResponse{
            Error:   "invalid_user_id",
            Message: "Invalid user ID format",
        })
        return
    }

    balance, err := ctrl.walletService.GetBalance(userID)
    if err != nil {
        c.JSON(http.StatusNotFound, models.ErrorResponse{
            Error:   "wallet_not_found",
            Message: err.Error(),
        })
        return
    }

    c.JSON(http.StatusOK, balance)
}

func (ctrl *WalletController) GetWallet(c *gin.Context) {
    userIDStr := c.GetString("user_id")
    userID, err := uuid.Parse(userIDStr)
    if err != nil {
        c.JSON(http.StatusBadRequest, models.ErrorResponse{
            Error:   "invalid_user_id",
            Message: "Invalid user ID format",
        })
        return
    }

    wallet, err := ctrl.walletService.GetWalletByUserID(userID)
    if err != nil {
        c.JSON(http.StatusNotFound, models.ErrorResponse{
            Error:   "wallet_not_found",
            Message: err.Error(),
        })
        return
    }

    c.JSON(http.StatusOK, wallet)
}