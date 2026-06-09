package services

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/utils"
	"github.com/mojobs/lara-payment-backend.git/pkg/money"
)

const defaultBuyerConfirmationWindowHours = 48

type EscrowService struct {
	db                      *gorm.DB
	userService             *UserService
	walletService           *WalletService
	transactionLimitService *TransactionLimitService
}

func NewEscrowService(db *gorm.DB, userService *UserService, walletService *WalletService, limitService *TransactionLimitService) *EscrowService {
	return &EscrowService{
		db:                      db,
		userService:             userService,
		walletService:           walletService,
		transactionLimitService: limitService,
	}
}

func (s *EscrowService) CreateOrder(sellerID uuid.UUID, req *models.CreateEscrowOrderRequest) (*models.EscrowOrderResponse, error) {
	if !req.Amount.IsPositive() {
		return nil, errors.New("amount must be greater than zero")
	}

	currency := strings.ToUpper(firstNonEmpty(req.Currency, "NGN"))
	ttlHours := req.BuyerConfirmationTTLHours
	if ttlHours == 0 {
		ttlHours = defaultBuyerConfirmationWindowHours
	}

	var metadata json.RawMessage
	if req.Metadata != nil {
		payload, err := json.Marshal(req.Metadata)
		if err != nil {
			return nil, err
		}
		metadata = payload
	}

	seller, err := s.userService.GetUserByID(sellerID.String())
	if err != nil {
		return nil, err
	}
	if !isUserKYCVerified(seller) {
		return nil, errors.New("complete Kora KYC before creating escrow orders")
	}

	sellerWallet, err := s.walletService.GetWalletByUserID(sellerID)
	if err != nil {
		return nil, err
	}
	if sellerWallet.Status != "ACTIVE" {
		return nil, errors.New("seller wallet is not active")
	}
	if sellerWallet.Currency != currency {
		return nil, fmt.Errorf("seller wallet currency %s does not match order currency %s", sellerWallet.Currency, currency)
	}

	order := models.EscrowOrder{
		Reference:            escrowReference(),
		SellerID:             sellerID,
		SellerWalletID:       sellerWallet.ID,
		Amount:               req.Amount,
		Currency:             currency,
		Title:                req.Title,
		Description:          req.Description,
		SalesChannel:         strings.ToUpper(req.SalesChannel),
		DeliveryMode:         strings.ToUpper(firstNonEmpty(req.DeliveryMode, "PHYSICAL")),
		Status:               "CREATED",
		BuyerConfirmationTTL: ttlHours,
		CrossBorder:          req.CrossBorder,
		SettlementCurrency:   strings.ToUpper(req.SettlementCurrency),
		FXLockedRate:         req.FXLockedRate,
		FXQuoteReference:     req.FXQuoteReference,
		Metadata:             metadata,
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		return s.recordEscrowEvent(tx, order.ID, &sellerID, "ORDER_CREATED", "Escrow order created", nil)
	}); err != nil {
		return nil, err
	}

	return s.GetOrder(sellerID, order.Reference)
}

func (s *EscrowService) ListOrders(userID uuid.UUID) ([]models.EscrowOrderResponse, error) {
	var orders []models.EscrowOrder
	if err := s.db.Where("seller_id = ? OR buyer_id = ?", userID, userID).
		Order("created_at DESC").
		Find(&orders).Error; err != nil {
		return nil, err
	}

	responses := make([]models.EscrowOrderResponse, 0, len(orders))
	for _, order := range orders {
		response, err := s.buildEscrowOrderResponse(&order)
		if err != nil {
			return nil, err
		}
		responses = append(responses, *response)
	}
	return responses, nil
}

func (s *EscrowService) GetOrder(userID uuid.UUID, reference string) (*models.EscrowOrderResponse, error) {
	order, err := s.getEscrowOrderForUser(userID, reference)
	if err != nil {
		return nil, err
	}
	return s.buildEscrowOrderResponse(order)
}

func (s *EscrowService) GetPublicOrder(reference string) (*models.EscrowPublicOrderResponse, error) {
	var order models.EscrowOrder
	if err := s.db.Where("reference = ?", reference).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("escrow order not found")
		}
		return nil, err
	}

	seller, err := s.userService.GetUserByID(order.SellerID.String())
	if err != nil {
		return nil, err
	}

	return &models.EscrowPublicOrderResponse{
		Reference:            order.Reference,
		SellerName:           strings.TrimSpace(strings.TrimSpace(seller.FirstName + " " + seller.LastName)),
		Title:                order.Title,
		Description:          order.Description,
		Amount:               order.Amount,
		Currency:             order.Currency,
		SalesChannel:         order.SalesChannel,
		DeliveryMode:         order.DeliveryMode,
		Status:               order.Status,
		BuyerConfirmationTTL: order.BuyerConfirmationTTL,
		CrossBorder:          order.CrossBorder,
		SettlementCurrency:   order.SettlementCurrency,
		FXLockedRate:         order.FXLockedRate,
		FXQuoteReference:     order.FXQuoteReference,
		CreatedAt:            order.CreatedAt,
	}, nil
}

func (s *EscrowService) FundOrder(buyerID uuid.UUID, reference string, req *models.FundEscrowOrderRequest) (*models.FundEscrowOrderResponse, error) {
	var order models.EscrowOrder
	var deliveryCode string
	var buyerWallet models.Wallet

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("reference = ?", reference).
			First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("escrow order not found")
			}
			return err
		}

		if order.Status != "CREATED" {
			return fmt.Errorf("escrow order cannot be funded from state %s", order.Status)
		}
		if order.SellerID == buyerID {
			return errors.New("seller cannot fund their own escrow order")
		}

		user, err := s.userService.GetUserByID(buyerID.String())
		if err != nil {
			return err
		}
		if err := utils.CheckPassword(user.PinHash, req.Pin); err != nil {
			return errors.New("invalid PIN")
		}
		buyerName := strings.TrimSpace(strings.TrimSpace(user.FirstName + " " + user.LastName))
		buyerEmail := ""
		if user.Email != nil {
			buyerEmail = *user.Email
		}

		if err := s.transactionLimitService.CheckAndRecordTransaction(tx, buyerID, order.Amount); err != nil {
			return err
		}

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner_type = ? AND user_id = ?", "USER", buyerID).
			First(&buyerWallet).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("buyer wallet not found")
			}
			return err
		}
		if buyerWallet.Status != "ACTIVE" {
			return errors.New("buyer wallet is not active")
		}
		if buyerWallet.Currency != order.Currency {
			return fmt.Errorf("buyer wallet currency %s does not match order currency %s", buyerWallet.Currency, order.Currency)
		}

		escrowWallet, err := s.walletService.getOrCreateSystemWallet(tx, escrowWalletReference(order.Currency), order.Currency)
		if err != nil {
			return err
		}

		if err := s.lockWalletsByID(tx, &buyerWallet, escrowWallet); err != nil {
			return err
		}

		if buyerWallet.Balance < order.Amount {
			return errors.New("insufficient funds")
		}

		if _, err := s.createLedgerTransfer(tx, &buyerWallet, escrowWallet, order.Amount, "ESCROW_FUND", fmt.Sprintf("Escrow funded for %s", order.Reference), map[string]interface{}{"escrow_reference": order.Reference}); err != nil {
			return err
		}

		deliveryCode, err = generateDeliveryCode()
		if err != nil {
			return err
		}
		hashedCode, err := utils.HashPassword(deliveryCode)
		if err != nil {
			return err
		}

		fundedAt := time.Now().UTC()
		order.BuyerID = &buyerID
		order.BuyerWalletID = &buyerWallet.ID
		order.EscrowWalletID = &escrowWallet.ID
		order.BuyerName = buyerName
		order.BuyerEmail = buyerEmail
		order.BuyerPhone = user.Phone
		order.Status = "FUNDED"
		order.DeliveryCodeHash = hashedCode
		order.FundedAt = &fundedAt

		if err := tx.Model(&order).Updates(map[string]interface{}{
			"buyer_id":              order.BuyerID,
			"buyer_wallet_id":       order.BuyerWalletID,
			"escrow_wallet_id":      order.EscrowWalletID,
			"buyer_name":            order.BuyerName,
			"buyer_email":           order.BuyerEmail,
			"buyer_phone":           order.BuyerPhone,
			"status":                order.Status,
			"delivery_code_hash":    order.DeliveryCodeHash,
			"funded_at":             fundedAt,
			"confirmation_deadline": nil,
		}).Error; err != nil {
			return err
		}

		return s.recordEscrowEvent(tx, order.ID, &buyerID, "ORDER_FUNDED", "Buyer funded escrow order", map[string]interface{}{"amount_kobo": order.Amount})
	})
	if err != nil {
		return nil, err
	}

	orderResponse, err := s.buildEscrowOrderResponse(&order)
	if err != nil {
		return nil, err
	}
	return &models.FundEscrowOrderResponse{
		Order:        *orderResponse,
		DeliveryCode: deliveryCode,
	}, nil
}

func (s *EscrowService) MarkShipped(sellerID uuid.UUID, reference string, req *models.ShipEscrowOrderRequest) (*models.EscrowOrderResponse, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order models.EscrowOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("reference = ?", reference).
			First(&order).Error; err != nil {
			return err
		}
		if order.SellerID != sellerID {
			return errors.New("you are not allowed to update this escrow order")
		}
		if order.Status != "FUNDED" {
			return fmt.Errorf("escrow order cannot be marked shipped from state %s", order.Status)
		}

		shippedAt := time.Now().UTC()
		updates := map[string]interface{}{
			"status":             "SHIPPED",
			"tracking_reference": req.TrackingReference,
			"delivery_proof_url": req.DeliveryProofURL,
			"shipped_at":         shippedAt,
		}
		if err := tx.Model(&order).Updates(updates).Error; err != nil {
			return err
		}
		return s.recordEscrowEvent(tx, order.ID, &sellerID, "ORDER_SHIPPED", firstNonEmpty(req.Note, "Seller marked order as shipped"), map[string]interface{}{"tracking_reference": req.TrackingReference})
	})
	if err != nil {
		return nil, err
	}
	return s.GetOrder(sellerID, reference)
}

func (s *EscrowService) MarkDelivered(sellerID uuid.UUID, reference string, req *models.MarkDeliveredEscrowOrderRequest) (*models.EscrowOrderResponse, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order models.EscrowOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("reference = ?", reference).
			First(&order).Error; err != nil {
			return err
		}
		if order.SellerID != sellerID {
			return errors.New("you are not allowed to update this escrow order")
		}
		if order.Status != "FUNDED" && order.Status != "SHIPPED" {
			return fmt.Errorf("escrow order cannot be marked delivered from state %s", order.Status)
		}

		deliveredAt := time.Now().UTC()
		confirmationDeadline := deliveredAt.Add(time.Duration(order.BuyerConfirmationTTL) * time.Hour)
		updates := map[string]interface{}{
			"status":                "DELIVERY_PENDING_CONFIRMATION",
			"delivery_proof_url":    firstNonEmpty(req.DeliveryProofURL, order.DeliveryProofURL),
			"delivered_at":          deliveredAt,
			"confirmation_deadline": confirmationDeadline,
		}
		if err := tx.Model(&order).Updates(updates).Error; err != nil {
			return err
		}
		return s.recordEscrowEvent(tx, order.ID, &sellerID, "ORDER_DELIVERED", firstNonEmpty(req.Note, "Seller marked order as delivered"), map[string]interface{}{"confirmation_deadline": confirmationDeadline})
	})
	if err != nil {
		return nil, err
	}
	return s.GetOrder(sellerID, reference)
}

func (s *EscrowService) ConfirmDelivery(buyerID uuid.UUID, reference string, req *models.ConfirmEscrowDeliveryRequest) (*models.EscrowOrderResponse, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order models.EscrowOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("reference = ?", reference).
			First(&order).Error; err != nil {
			return err
		}
		if order.BuyerID == nil || *order.BuyerID != buyerID {
			return errors.New("you are not allowed to confirm this escrow order")
		}
		if order.Status != "DELIVERY_PENDING_CONFIRMATION" {
			return fmt.Errorf("escrow order cannot be confirmed from state %s", order.Status)
		}
		switch {
		case req.DeliveryCode != "":
			if order.DeliveryCodeHash == "" {
				return errors.New("delivery code is not available for this escrow order")
			}
			if err := utils.CheckPassword(order.DeliveryCodeHash, req.DeliveryCode); err != nil {
				return errors.New("invalid delivery code")
			}
		case req.Pin != "":
			user, err := s.userService.GetUserByID(buyerID.String())
			if err != nil {
				return err
			}
			if err := utils.CheckPassword(user.PinHash, req.Pin); err != nil {
				return errors.New("invalid PIN")
			}
		default:
			return errors.New("delivery code or PIN is required")
		}
		return s.releaseEscrowTx(tx, &order, &buyerID, "BUYER_CONFIRMED", "Buyer confirmed delivery")
	})
	if err != nil {
		return nil, err
	}
	return s.GetOrder(buyerID, reference)
}

func (s *EscrowService) ReleaseIfEligible(userID uuid.UUID, reference string) (*models.EscrowOrderResponse, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order models.EscrowOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("reference = ?", reference).
			First(&order).Error; err != nil {
			return err
		}
		if order.SellerID != userID && (order.BuyerID == nil || *order.BuyerID != userID) {
			return errors.New("you are not allowed to release this escrow order")
		}
		if order.Status != "DELIVERY_PENDING_CONFIRMATION" {
			return fmt.Errorf("escrow order cannot be auto released from state %s", order.Status)
		}
		if order.ConfirmationDeadline == nil || order.ConfirmationDeadline.After(time.Now().UTC()) {
			return errors.New("escrow order is not yet eligible for auto release")
		}
		return s.releaseEscrowTx(tx, &order, nil, "AUTO_RELEASED", "Escrow auto released after buyer confirmation window")
	})
	if err != nil {
		return nil, err
	}
	return s.GetOrder(userID, reference)
}

func (s *EscrowService) OpenDispute(userID uuid.UUID, reference string, req *models.OpenEscrowDisputeRequest) (*models.EscrowOrderResponse, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order models.EscrowOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("reference = ?", reference).
			First(&order).Error; err != nil {
			return err
		}
		if order.BuyerID == nil || *order.BuyerID != userID {
			return errors.New("only the buyer can open a dispute")
		}
		if order.Status != "FUNDED" && order.Status != "SHIPPED" && order.Status != "DELIVERY_PENDING_CONFIRMATION" {
			return fmt.Errorf("escrow order cannot be disputed from state %s", order.Status)
		}

		disputedAt := time.Now().UTC()
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"status":      "DISPUTED",
			"disputed_at": disputedAt,
		}).Error; err != nil {
			return err
		}

		dispute := models.EscrowDispute{
			EscrowID:       order.ID,
			RaisedByUserID: userID,
			Reason:         req.Reason,
			Details:        req.Details,
			EvidenceURL:    req.EvidenceURL,
			Status:         "OPEN",
		}
		if err := tx.Create(&dispute).Error; err != nil {
			return err
		}
		return s.recordEscrowEvent(tx, order.ID, &userID, "DISPUTE_OPENED", req.Reason, map[string]interface{}{"evidence_url": req.EvidenceURL})
	})
	if err != nil {
		return nil, err
	}
	return s.GetOrder(userID, reference)
}

func (s *EscrowService) CancelOrder(sellerID uuid.UUID, reference string, req *models.CancelEscrowOrderRequest) (*models.EscrowOrderResponse, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order models.EscrowOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("reference = ?", reference).
			First(&order).Error; err != nil {
			return err
		}
		if order.SellerID != sellerID {
			return errors.New("only the seller can cancel this escrow order")
		}
		if order.Status != "CREATED" {
			return fmt.Errorf("escrow order cannot be cancelled from state %s", order.Status)
		}

		cancelledAt := time.Now().UTC()
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"status":       "CANCELLED",
			"cancelled_at": cancelledAt,
		}).Error; err != nil {
			return err
		}
		return s.recordEscrowEvent(tx, order.ID, &sellerID, "ORDER_CANCELLED", firstNonEmpty(req.Reason, "Seller cancelled escrow order"), nil)
	})
	if err != nil {
		return nil, err
	}
	return s.GetOrder(sellerID, reference)
}

func (s *EscrowService) RefundOrder(sellerID uuid.UUID, reference string, req *models.RefundEscrowOrderRequest) (*models.EscrowOrderResponse, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order models.EscrowOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("reference = ?", reference).
			First(&order).Error; err != nil {
			return err
		}
		if order.SellerID != sellerID {
			return errors.New("only the seller can refund this escrow order")
		}
		if order.Status != "FUNDED" && order.Status != "SHIPPED" && order.Status != "DELIVERY_PENDING_CONFIRMATION" && order.Status != "DISPUTED" {
			return fmt.Errorf("escrow order cannot be refunded from state %s", order.Status)
		}
		return s.refundEscrowTx(tx, &order, &sellerID, "ORDER_REFUNDED", firstNonEmpty(req.Reason, "Seller refunded escrow order"))
	})
	if err != nil {
		return nil, err
	}
	return s.GetOrder(sellerID, reference)
}

func (s *EscrowService) FundOrderFromProviderTx(tx *gorm.DB, orderID uuid.UUID, providerTx *models.ProviderTransaction) error {
	var order models.EscrowOrder
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", orderID).First(&order).Error; err != nil {
		return err
	}
	if order.Status == "FUNDED" || order.Status == "SHIPPED" || order.Status == "DELIVERY_PENDING_CONFIRMATION" || order.Status == "RELEASED" || order.Status == "REFUNDED" {
		return nil
	}
	if order.Status != "CREATED" {
		return fmt.Errorf("escrow order cannot be provider-funded from state %s", order.Status)
	}

	metadata := escrowPayinMetadataFromProviderTx(providerTx)
	if providerTx.UserID != nil && *providerTx.UserID == order.SellerID {
		return errors.New("seller cannot be recorded as buyer for provider-funded escrow")
	}
	if providerTx.UserID == nil && order.BuyerID != nil {
		return errors.New("external provider payment cannot fund an order with a registered buyer")
	}
	if providerTx.UserID != nil && order.BuyerID != nil && *order.BuyerID != *providerTx.UserID {
		return errors.New("provider payment buyer does not match committed escrow buyer")
	}

	var buyerWallet *models.Wallet
	var buyerName string
	var buyerEmail string
	var buyerPhone string
	var actorUserID *uuid.UUID
	if providerTx.UserID != nil {
		var wallet models.Wallet
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner_type = ? AND user_id = ?", "USER", *providerTx.UserID).
			First(&wallet).Error; err != nil {
			return err
		}
		if wallet.Currency != order.Currency {
			return fmt.Errorf("buyer wallet currency %s does not match order currency %s", wallet.Currency, order.Currency)
		}

		buyer, err := s.userService.GetUserByID(providerTx.UserID.String())
		if err != nil {
			return err
		}
		buyerWallet = &wallet
		actorUserID = providerTx.UserID
		buyerName = firstNonEmpty(metadata.BuyerName, strings.TrimSpace(buyer.FirstName+" "+buyer.LastName))
		if buyer.Email != nil {
			buyerEmail = firstNonEmpty(metadata.BuyerEmail, *buyer.Email)
		} else {
			buyerEmail = metadata.BuyerEmail
		}
		buyerPhone = firstNonEmpty(metadata.BuyerPhone, buyer.Phone)
	} else {
		buyerName = firstNonEmpty(metadata.BuyerName, order.BuyerName)
		buyerEmail = firstNonEmpty(metadata.BuyerEmail, order.BuyerEmail)
		buyerPhone = firstNonEmpty(metadata.BuyerPhone, order.BuyerPhone)
	}

	railWallet, err := s.walletService.getOrCreateSystemWallet(tx, koraInflowWalletReference(order.Currency), order.Currency)
	if err != nil {
		return err
	}
	escrowWallet, err := s.walletService.getOrCreateSystemWallet(tx, escrowWalletReference(order.Currency), order.Currency)
	if err != nil {
		return err
	}

	if err := s.creditExternalWalletTx(tx, railWallet, order.Amount, "ESCROW_PROVIDER_PAYIN", fmt.Sprintf("Kora escrow funding for %s", order.Reference), map[string]interface{}{
		"provider":           providerTx.Provider,
		"provider_reference": providerTx.Reference,
		"escrow_reference":   order.Reference,
	}); err != nil {
		return err
	}
	if err := s.lockWalletsByID(tx, railWallet, escrowWallet); err != nil {
		return err
	}
	if _, err := s.createLedgerTransfer(tx, railWallet, escrowWallet, order.Amount, "ESCROW_FUND", fmt.Sprintf("Escrow funded from provider for %s", order.Reference), map[string]interface{}{
		"provider":           providerTx.Provider,
		"provider_reference": providerTx.Reference,
		"escrow_reference":   order.Reference,
		"checkout_source":    metadata.CheckoutSource,
	}); err != nil {
		return err
	}

	fundedAt := time.Now().UTC()
	order.EscrowWalletID = &escrowWallet.ID
	order.BuyerName = buyerName
	order.BuyerEmail = buyerEmail
	order.BuyerPhone = buyerPhone
	order.Status = "FUNDED"
	order.FundedAt = &fundedAt
	if providerTx.UserID != nil {
		order.BuyerID = providerTx.UserID
		order.BuyerWalletID = &buyerWallet.ID
	}

	updates := map[string]interface{}{
		"escrow_wallet_id":      order.EscrowWalletID,
		"buyer_name":            order.BuyerName,
		"buyer_email":           order.BuyerEmail,
		"buyer_phone":           order.BuyerPhone,
		"status":                order.Status,
		"funded_at":             fundedAt,
		"confirmation_deadline": nil,
	}
	if providerTx.UserID != nil {
		updates["buyer_id"] = order.BuyerID
		updates["buyer_wallet_id"] = order.BuyerWalletID
	}
	if err := tx.Model(&order).Updates(updates).Error; err != nil {
		return err
	}

	return s.recordEscrowEvent(tx, order.ID, actorUserID, "ORDER_FUNDED", "Buyer funded escrow order through Kora checkout", map[string]interface{}{
		"provider":           providerTx.Provider,
		"provider_reference": providerTx.Reference,
		"amount_kobo":        order.Amount,
		"checkout_source":    metadata.CheckoutSource,
		"buyer_email":        buyerEmail,
	})
}

type escrowProviderPayinMetadata struct {
	CheckoutSource string `json:"checkout_source"`
	BuyerName      string `json:"buyer_name"`
	BuyerEmail     string `json:"buyer_email"`
	BuyerPhone     string `json:"buyer_phone"`
}

func escrowPayinMetadataFromProviderTx(providerTx *models.ProviderTransaction) escrowProviderPayinMetadata {
	var metadata escrowProviderPayinMetadata
	if providerTx == nil || len(providerTx.RequestPayload) == 0 {
		return metadata
	}
	_ = json.Unmarshal(providerTx.RequestPayload, &metadata)
	return metadata
}

func (s *EscrowService) ResolveDispute(reference string, req *models.ResolveEscrowDisputeRequest) (*models.EscrowOrderResponse, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order models.EscrowOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("reference = ?", reference).
			First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("escrow order not found")
			}
			return err
		}
		if order.Status != "DISPUTED" {
			return fmt.Errorf("escrow order cannot be dispute-resolved from state %s", order.Status)
		}

		switch req.Action {
		case "RELEASE":
			return s.releaseEscrowTx(tx, &order, nil, "DISPUTE_RELEASED", firstNonEmpty(req.ResolutionNote, "Admin released escrow after dispute review"))
		case "REFUND":
			return s.refundEscrowTx(tx, &order, nil, "DISPUTE_REFUNDED", firstNonEmpty(req.ResolutionNote, "Admin refunded escrow after dispute review"))
		default:
			return errors.New("unsupported dispute action")
		}
	})
	if err != nil {
		return nil, err
	}

	var order models.EscrowOrder
	if err := s.db.Where("reference = ?", reference).First(&order).Error; err != nil {
		return nil, err
	}
	return s.buildEscrowOrderResponse(&order)
}

func (s *EscrowService) getEscrowOrderForUser(userID uuid.UUID, reference string) (*models.EscrowOrder, error) {
	var order models.EscrowOrder
	if err := s.db.Where("reference = ? AND (seller_id = ? OR buyer_id = ?)", reference, userID, userID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("escrow order not found")
		}
		return nil, err
	}
	return &order, nil
}

func (s *EscrowService) buildEscrowOrderResponse(order *models.EscrowOrder) (*models.EscrowOrderResponse, error) {
	var events []models.EscrowEvent
	if err := s.db.Where("escrow_id = ?", order.ID).Order("created_at ASC").Find(&events).Error; err != nil {
		return nil, err
	}
	var disputes []models.EscrowDispute
	if err := s.db.Where("escrow_id = ?", order.ID).Order("created_at DESC").Find(&disputes).Error; err != nil {
		return nil, err
	}

	response := &models.EscrowOrderResponse{
		ID:                   order.ID.String(),
		Reference:            order.Reference,
		SellerID:             order.SellerID.String(),
		BuyerName:            order.BuyerName,
		BuyerEmail:           order.BuyerEmail,
		BuyerPhone:           order.BuyerPhone,
		Amount:               order.Amount,
		Currency:             order.Currency,
		Title:                order.Title,
		Description:          order.Description,
		SalesChannel:         order.SalesChannel,
		DeliveryMode:         order.DeliveryMode,
		Status:               order.Status,
		TrackingReference:    order.TrackingReference,
		DeliveryProofURL:     order.DeliveryProofURL,
		BuyerConfirmationTTL: order.BuyerConfirmationTTL,
		CrossBorder:          order.CrossBorder,
		SettlementCurrency:   order.SettlementCurrency,
		FXLockedRate:         order.FXLockedRate,
		FXQuoteReference:     order.FXQuoteReference,
		FundedAt:             order.FundedAt,
		ShippedAt:            order.ShippedAt,
		DeliveredAt:          order.DeliveredAt,
		ConfirmationDeadline: order.ConfirmationDeadline,
		ReleasedAt:           order.ReleasedAt,
		DisputedAt:           order.DisputedAt,
		CancelledAt:          order.CancelledAt,
		RefundedAt:           order.RefundedAt,
		CreatedAt:            order.CreatedAt,
		UpdatedAt:            order.UpdatedAt,
		Events:               make([]models.EscrowEventDTO, 0, len(events)),
		Disputes:             make([]models.EscrowDisputeDTO, 0, len(disputes)),
	}
	if order.BuyerID != nil {
		response.BuyerID = order.BuyerID.String()
	}
	for _, event := range events {
		response.Events = append(response.Events, models.EscrowEventDTO{
			Action:    event.Action,
			Note:      event.Note,
			CreatedAt: event.CreatedAt,
		})
	}
	for _, dispute := range disputes {
		response.Disputes = append(response.Disputes, models.EscrowDisputeDTO{
			ID:          dispute.ID.String(),
			Reason:      dispute.Reason,
			Details:     dispute.Details,
			EvidenceURL: dispute.EvidenceURL,
			Status:      dispute.Status,
			CreatedAt:   dispute.CreatedAt,
			ResolvedAt:  dispute.ResolvedAt,
		})
	}
	return response, nil
}

func (s *EscrowService) releaseEscrowTx(tx *gorm.DB, order *models.EscrowOrder, actorUserID *uuid.UUID, action, note string) error {
	if order.EscrowWalletID == nil {
		return errors.New("escrow wallet is not set")
	}
	var escrowWallet, sellerWallet models.Wallet
	if err := tx.Where("id = ?", *order.EscrowWalletID).First(&escrowWallet).Error; err != nil {
		return err
	}
	if err := tx.Where("id = ?", order.SellerWalletID).First(&sellerWallet).Error; err != nil {
		return err
	}
	if err := s.lockWalletsByID(tx, &escrowWallet, &sellerWallet); err != nil {
		return err
	}
	if escrowWallet.Balance < order.Amount {
		return errors.New("escrow wallet has insufficient balance for release")
	}

	if _, err := s.createLedgerTransfer(tx, &escrowWallet, &sellerWallet, order.Amount, "ESCROW_RELEASE", fmt.Sprintf("Escrow released for %s", order.Reference), map[string]interface{}{"escrow_reference": order.Reference}); err != nil {
		return err
	}

	releasedAt := time.Now().UTC()
	if err := tx.Model(order).Updates(map[string]interface{}{
		"status":      "RELEASED",
		"released_at": releasedAt,
	}).Error; err != nil {
		return err
	}
	if err := s.closeOpenEscrowDisputesTx(tx, order.ID, "RESOLVED_RELEASED"); err != nil {
		return err
	}
	return s.recordEscrowEvent(tx, order.ID, actorUserID, action, note, nil)
}

func (s *EscrowService) refundEscrowTx(tx *gorm.DB, order *models.EscrowOrder, actorUserID *uuid.UUID, action, note string) error {
	if order.EscrowWalletID == nil {
		return errors.New("escrow order is not fully funded")
	}
	if order.BuyerID == nil || order.BuyerWalletID == nil {
		return errors.New("escrow order was funded through external Kora checkout; initiate a Kora refund instead")
	}

	var escrowWallet, buyerWallet models.Wallet
	if err := tx.Where("id = ?", *order.EscrowWalletID).First(&escrowWallet).Error; err != nil {
		return err
	}
	if err := tx.Where("id = ?", *order.BuyerWalletID).First(&buyerWallet).Error; err != nil {
		return err
	}
	if err := s.lockWalletsByID(tx, &escrowWallet, &buyerWallet); err != nil {
		return err
	}
	if escrowWallet.Balance < order.Amount {
		return errors.New("escrow wallet has insufficient balance for refund")
	}

	if _, err := s.createLedgerTransfer(tx, &escrowWallet, &buyerWallet, order.Amount, "ESCROW_REFUND", fmt.Sprintf("Escrow refunded for %s", order.Reference), map[string]interface{}{"escrow_reference": order.Reference}); err != nil {
		return err
	}

	refundedAt := time.Now().UTC()
	if err := tx.Model(order).Updates(map[string]interface{}{
		"status":      "REFUNDED",
		"refunded_at": refundedAt,
	}).Error; err != nil {
		return err
	}
	if err := s.closeOpenEscrowDisputesTx(tx, order.ID, "RESOLVED_REFUNDED"); err != nil {
		return err
	}
	return s.recordEscrowEvent(tx, order.ID, actorUserID, action, note, nil)
}

func (s *EscrowService) closeOpenEscrowDisputesTx(tx *gorm.DB, escrowID uuid.UUID, status string) error {
	resolvedAt := time.Now().UTC()
	return tx.Model(&models.EscrowDispute{}).
		Where("escrow_id = ? AND status = ?", escrowID, "OPEN").
		Updates(map[string]interface{}{
			"status":      status,
			"resolved_at": resolvedAt,
		}).Error
}

func (s *EscrowService) creditExternalWalletTx(tx *gorm.DB, wallet *models.Wallet, amount money.Amount, transactionType, description string, metadata interface{}) error {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	transaction := models.Transaction{
		Reference:   ledgerReference(transactionType),
		ToWalletID:  &wallet.ID,
		Amount:      amount,
		Currency:    wallet.Currency,
		Type:        transactionType,
		Status:      "PENDING",
		Description: description,
		Metadata:    payload,
	}
	if err := tx.Create(&transaction).Error; err != nil {
		return err
	}

	newBalance := wallet.Balance + amount
	if err := tx.Model(wallet).Update("balance", newBalance).Error; err != nil {
		return err
	}
	wallet.Balance = newBalance

	entry := models.LedgerEntry{
		TransactionID: transaction.ID,
		WalletID:      wallet.ID,
		Debit:         money.Zero,
		Credit:        amount,
		BalanceAfter:  newBalance,
	}
	if err := tx.Create(&entry).Error; err != nil {
		return err
	}

	completedAt := time.Now().UTC()
	return tx.Model(&transaction).Updates(map[string]interface{}{
		"status":       "COMPLETED",
		"completed_at": completedAt,
	}).Error
}

func (s *EscrowService) recordEscrowEvent(tx *gorm.DB, escrowID uuid.UUID, actorUserID *uuid.UUID, action, note string, metadata interface{}) error {
	var payload json.RawMessage
	if metadata != nil {
		bytes, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		payload = bytes
	}
	event := models.EscrowEvent{
		EscrowID:    escrowID,
		ActorUserID: actorUserID,
		Action:      action,
		Note:        note,
		Metadata:    payload,
	}
	return tx.Create(&event).Error
}

func (s *EscrowService) lockWalletsByID(tx *gorm.DB, first, second *models.Wallet) error {
	lock := clause.Locking{Strength: "UPDATE"}
	if first.ID.String() < second.ID.String() {
		if err := tx.Clauses(lock).Where("id = ?", first.ID).First(first).Error; err != nil {
			return err
		}
		return tx.Clauses(lock).Where("id = ?", second.ID).First(second).Error
	}
	if err := tx.Clauses(lock).Where("id = ?", second.ID).First(second).Error; err != nil {
		return err
	}
	return tx.Clauses(lock).Where("id = ?", first.ID).First(first).Error
}

func (s *EscrowService) createLedgerTransfer(tx *gorm.DB, fromWallet, toWallet *models.Wallet, amount money.Amount, transactionType, description string, metadata interface{}) (*models.Transaction, error) {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	transaction := models.Transaction{
		Reference:    ledgerReference(transactionType),
		FromWalletID: &fromWallet.ID,
		ToWalletID:   &toWallet.ID,
		Amount:       amount,
		Currency:     fromWallet.Currency,
		Type:         transactionType,
		Status:       "PENDING",
		Description:  description,
		Metadata:     payload,
	}
	if err := tx.Create(&transaction).Error; err != nil {
		return nil, err
	}

	newFromBalance := fromWallet.Balance - amount
	newToBalance := toWallet.Balance + amount
	if err := tx.Model(fromWallet).Update("balance", newFromBalance).Error; err != nil {
		return nil, err
	}
	if err := tx.Model(toWallet).Update("balance", newToBalance).Error; err != nil {
		return nil, err
	}
	fromWallet.Balance = newFromBalance
	toWallet.Balance = newToBalance

	entries := []models.LedgerEntry{
		{
			TransactionID: transaction.ID,
			WalletID:      fromWallet.ID,
			Debit:         amount,
			Credit:        money.Zero,
			BalanceAfter:  newFromBalance,
		},
		{
			TransactionID: transaction.ID,
			WalletID:      toWallet.ID,
			Debit:         money.Zero,
			Credit:        amount,
			BalanceAfter:  newToBalance,
		},
	}
	if err := tx.Create(&entries).Error; err != nil {
		return nil, err
	}

	completedAt := time.Now().UTC()
	if err := tx.Model(&transaction).Updates(map[string]interface{}{
		"status":       "COMPLETED",
		"completed_at": completedAt,
	}).Error; err != nil {
		return nil, err
	}
	transaction.Status = "COMPLETED"
	transaction.CompletedAt = &completedAt
	return &transaction, nil
}

func escrowWalletReference(currency string) string {
	return "ESCROW_" + strings.ToUpper(currency)
}

func koraInflowWalletReference(currency string) string {
	return "KORA_INFLOW_" + strings.ToUpper(currency)
}

func escrowReference() string {
	return fmt.Sprintf("ESC-%s", uuid.New().String()[:12])
}

func ledgerReference(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, uuid.New().String()[:12])
}

func generateDeliveryCode() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
