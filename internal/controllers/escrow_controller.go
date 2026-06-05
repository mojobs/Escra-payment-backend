package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/services"
)

type EscrowController struct {
	escrowService   *services.EscrowService
	providerService *services.ProviderService
	publicBaseURL   string
}

func NewEscrowController(escrowService *services.EscrowService, providerService *services.ProviderService, publicBaseURL string) *EscrowController {
	return &EscrowController{
		escrowService:   escrowService,
		providerService: providerService,
		publicBaseURL:   publicBaseURL,
	}
}

func (ctrl *EscrowController) CreateOrder(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var req models.CreateEscrowOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	order, err := ctrl.escrowService.CreateOrder(userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "escrow_create_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, order)
}

func (ctrl *EscrowController) ListOrders(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	orders, err := ctrl.escrowService.ListOrders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "escrow_list_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"orders":  orders,
		"count":   len(orders),
	})
}

func (ctrl *EscrowController) GetOrder(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	order, err := ctrl.escrowService.GetOrder(userID, c.Param("reference"))
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "escrow_not_found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

func (ctrl *EscrowController) GetPublicOrder(c *gin.Context) {
	order, err := ctrl.escrowService.GetPublicOrder(c.Param("reference"))
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "escrow_not_found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

func (ctrl *EscrowController) FundOrder(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var req models.FundEscrowOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	response, err := ctrl.escrowService.FundOrder(userID, c.Param("reference"), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "escrow_fund_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (ctrl *EscrowController) InitiateKoraCheckout(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var req models.KoraEscrowCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	response, err := ctrl.providerService.InitiateEscrowKoraCheckout(c.Request.Context(), userID, c.Param("reference"), &req, ctrl.publicBaseURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "escrow_checkout_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, response)
}

func (ctrl *EscrowController) MarkShipped(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var req models.ShipEscrowOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	order, err := ctrl.escrowService.MarkShipped(userID, c.Param("reference"), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "escrow_ship_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

func (ctrl *EscrowController) MarkDelivered(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var req models.MarkDeliveredEscrowOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	order, err := ctrl.escrowService.MarkDelivered(userID, c.Param("reference"), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "escrow_mark_delivered_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

func (ctrl *EscrowController) ConfirmDelivery(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var req models.ConfirmEscrowDeliveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	order, err := ctrl.escrowService.ConfirmDelivery(userID, c.Param("reference"), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "escrow_confirm_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

func (ctrl *EscrowController) ReleaseIfEligible(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	order, err := ctrl.escrowService.ReleaseIfEligible(userID, c.Param("reference"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "escrow_release_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

func (ctrl *EscrowController) OpenDispute(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var req models.OpenEscrowDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	order, err := ctrl.escrowService.OpenDispute(userID, c.Param("reference"), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "escrow_dispute_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

func (ctrl *EscrowController) CancelOrder(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var req models.CancelEscrowOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	order, err := ctrl.escrowService.CancelOrder(userID, c.Param("reference"), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "escrow_cancel_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

func (ctrl *EscrowController) RefundOrder(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var req models.RefundEscrowOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	order, err := ctrl.escrowService.RefundOrder(userID, c.Param("reference"), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "escrow_refund_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

func (ctrl *EscrowController) ResolveDispute(c *gin.Context) {
	var req models.ResolveEscrowDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	order, err := ctrl.escrowService.ResolveDispute(c.Param("reference"), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "escrow_dispute_resolution_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}
