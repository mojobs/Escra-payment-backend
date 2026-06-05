package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/services"
)

type ProviderController struct {
	providerService *services.ProviderService
}

func NewProviderController(providerService *services.ProviderService) *ProviderController {
	return &ProviderController{providerService: providerService}
}

func (ctrl *ProviderController) ListKoraBanks(c *gin.Context) {
	countryCode := c.DefaultQuery("countryCode", "NG")
	banks, err := ctrl.providerService.ListKoraBanks(c.Request.Context(), countryCode)
	if err != nil {
		c.JSON(http.StatusBadGateway, models.ErrorResponse{
			Error:   "kora_banks_failed",
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "banks": banks})
}

func (ctrl *ProviderController) ResolveKoraBankAccount(c *gin.Context) {
	bankCode := c.Query("bank_code")
	accountNumber := c.Query("account_number")
	currency := c.DefaultQuery("currency", "NG")
	if bankCode == "" || accountNumber == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: "bank_code and account_number are required",
		})
		return
	}

	account, err := ctrl.providerService.ResolveKoraBankAccount(c.Request.Context(), bankCode, accountNumber, currency)
	if err != nil {
		c.JSON(http.StatusBadGateway, models.ErrorResponse{
			Error:   "kora_account_resolve_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.KoraBankResolveResponse{
		BankName:      account.BankName,
		BankCode:      account.BankCode,
		AccountNumber: account.AccountNumber,
		AccountName:   account.AccountName,
	})
}

func (ctrl *ProviderController) InitiateKoraBankPayout(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var req models.KoraBankPayoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	providerTx, err := ctrl.providerService.InitiateKoraBankPayout(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "kora_payout_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, providerTransactionResponse(providerTx))
}

func (ctrl *ProviderController) InitiateQuidaxWithdrawal(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var req models.QuidaxCryptoWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	providerTx, err := ctrl.providerService.InitiateQuidaxWithdrawal(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "quidax_withdrawal_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, providerTransactionResponse(providerTx))
}

func authenticatedUserID(c *gin.Context) (uuid.UUID, bool) {
	userIDStr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_user_id",
			Message: "Invalid user ID format",
		})
		return uuid.Nil, false
	}
	return userID, true
}

func providerTransactionResponse(tx *models.ProviderTransaction) models.ProviderTransactionResponse {
	return models.ProviderTransactionResponse{
		ID:                tx.ID.String(),
		Provider:          tx.Provider,
		Reference:         tx.Reference,
		ExternalReference: tx.ExternalReference,
		Type:              tx.Type,
		Status:            tx.Status,
		Amount:            tx.Amount,
		AmountText:        tx.AmountText,
		Currency:          tx.Currency,
		CreatedAt:         tx.CreatedAt,
	}
}
