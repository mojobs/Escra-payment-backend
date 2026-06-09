package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/providers/kora"
	"github.com/mojobs/lara-payment-backend.git/internal/providers/quidax"
	"github.com/mojobs/lara-payment-backend.git/internal/utils"
	"github.com/mojobs/lara-payment-backend.git/pkg/money"
)

type KoraClient interface {
	ListBanks(ctx context.Context, countryCode string) ([]kora.Bank, error)
	ResolveBankAccount(ctx context.Context, req kora.ResolveBankAccountRequest) (*kora.ResolvedBankAccount, error)
	VerifyIdentity(ctx context.Context, req kora.VerifyIdentityRequest) (*kora.VerifyIdentityResponse, error)
	CreateVirtualAccount(ctx context.Context, req kora.VirtualAccountRequest) (*kora.VirtualAccountResponse, error)
	GetBalances(ctx context.Context) ([]kora.Balance, error)
	RequestPayout(ctx context.Context, req kora.PayoutRequest) (*kora.PayoutResponse, error)
	RequestRefund(ctx context.Context, req kora.RefundRequest) (*kora.RefundResponse, error)
	InitializeCheckout(ctx context.Context, req kora.CheckoutRequest) (*kora.CheckoutResponse, error)
}

type QuidaxClient interface {
	Withdraw(ctx context.Context, req quidax.WithdrawRequest) (*quidax.WithdrawResponse, error)
	FetchWithdrawal(ctx context.Context, userID, withdrawalID string) (*quidax.WithdrawalDetail, error)
}

type ProviderService struct {
	db                      *gorm.DB
	userService             *UserService
	walletService           *WalletService
	transactionLimitService *TransactionLimitService
	koraClient              KoraClient
	quidaxClient            QuidaxClient
}

func NewProviderService(db *gorm.DB, userService *UserService, walletService *WalletService, limitService *TransactionLimitService, koraClient KoraClient, quidaxClient QuidaxClient) *ProviderService {
	return &ProviderService{
		db:                      db,
		userService:             userService,
		walletService:           walletService,
		transactionLimitService: limitService,
		koraClient:              koraClient,
		quidaxClient:            quidaxClient,
	}
}

func (s *ProviderService) ListKoraBanks(ctx context.Context, countryCode string) ([]kora.Bank, error) {
	return s.koraClient.ListBanks(ctx, countryCode)
}

func (s *ProviderService) ResolveKoraBankAccount(ctx context.Context, bankCode, accountNumber, currency string) (*kora.ResolvedBankAccount, error) {
	return s.koraClient.ResolveBankAccount(ctx, kora.ResolveBankAccountRequest{
		BankCode:      bankCode,
		AccountNumber: accountNumber,
		Currency:      currency,
	})
}

func (s *ProviderService) VerifyKoraIdentity(ctx context.Context, userID uuid.UUID, req *models.KoraVerifyIdentityRequest) (*models.KoraVerifyIdentityResponse, error) {
	idType := strings.ToUpper(strings.TrimSpace(req.IDType))
	verifiedAt := timeNowUTC()

	response, err := s.koraClient.VerifyIdentity(ctx, kora.VerifyIdentityRequest{
		IDNumber: req.IDNumber,
		IDType:   idType,
	})
	if err != nil {
		return nil, err
	}

	kycStatus := "VERIFIED"
	if normalized := normalizeProviderStatus(response.Status); normalized == "FAILED" {
		kycStatus = "FAILED"
	}

	payload := response.Raw
	updates := map[string]interface{}{
		"kyc_status":      kycStatus,
		"kyc_provider":    "kora",
		"kyc_reference":   firstNonEmpty(response.Reference, providerReference("KYC")),
		"kyc_id_type":     idType,
		"kyc_id_last4":    last4(req.IDNumber),
		"kyc_response":    json.RawMessage(payload),
		"kyc_verified_at": nil,
	}
	if kycStatus == "VERIFIED" {
		updates["kyc_verified_at"] = verifiedAt
	}
	if err := s.db.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return nil, err
	}

	var verifiedAtPtr *time.Time
	if kycStatus == "VERIFIED" {
		verifiedAtPtr = &verifiedAt
	}
	return &models.KoraVerifyIdentityResponse{
		Success:           kycStatus == "VERIFIED",
		KYCStatus:         kycStatus,
		KYCProvider:       "kora",
		KYCReference:      updates["kyc_reference"].(string),
		KYCIDType:         idType,
		KYCIDLast4:        last4(req.IDNumber),
		KYCVerifiedAt:     verifiedAtPtr,
		ProviderMessage:   response.Message,
		ProviderReference: response.Reference,
	}, nil
}

func (s *ProviderService) CreateKoraVirtualAccount(ctx context.Context, userID uuid.UUID, req *models.KoraVirtualAccountRequest) (*models.KoraVirtualAccountResponse, error) {
	user, err := s.userService.GetUserByID(userID.String())
	if err != nil {
		return nil, err
	}
	if !isUserKYCVerified(user) {
		return nil, errors.New("complete Kora KYC before creating a virtual account")
	}
	if user.Email == nil || strings.TrimSpace(*user.Email) == "" {
		return nil, errors.New("email is required to create a Kora virtual account")
	}

	currency := strings.ToUpper(firstNonEmpty(req.Currency, "NGN"))
	accountName := firstNonEmpty(req.AccountName, strings.TrimSpace(user.FirstName+" "+user.LastName))
	accountReference := providerReference("VBA")

	var order *models.EscrowOrder
	if req.EscrowReference != "" {
		var found models.EscrowOrder
		if err := s.db.Where("reference = ?", req.EscrowReference).First(&found).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("escrow order not found")
			}
			return nil, err
		}
		if found.Status != "CREATED" {
			return nil, fmt.Errorf("escrow order cannot receive a virtual account from state %s", found.Status)
		}
		if found.SellerID == userID {
			return nil, errors.New("seller cannot fund their own escrow order")
		}
		if found.BuyerID != nil && *found.BuyerID != userID {
			return nil, errors.New("another buyer has already started funding this escrow order")
		}
		if found.Currency != currency {
			return nil, fmt.Errorf("escrow currency %s does not match virtual account currency %s", found.Currency, currency)
		}
		order = &found
	}

	requestPayload, err := json.Marshal(map[string]interface{}{
		"account_reference": accountReference,
		"account_name":      accountName,
		"bank_code":         req.BankCode,
		"currency":          currency,
		"permanent":         req.Permanent,
		"escrow_reference":  req.EscrowReference,
		"id_type":           req.IDType,
		"id_last4":          last4(req.IDNumber),
	})
	if err != nil {
		return nil, err
	}

	var providerTx models.ProviderTransaction
	var virtualAccount models.KoraVirtualAccount
	err = s.db.Transaction(func(tx *gorm.DB) error {
		providerTx = models.ProviderTransaction{
			Provider:       "kora",
			Reference:      accountReference,
			UserID:         &userID,
			Type:           "ESCROW_VIRTUAL_ACCOUNT",
			Status:         "PENDING",
			Currency:       currency,
			RequestPayload: json.RawMessage(requestPayload),
		}
		if order != nil {
			providerTx.EscrowOrderID = &order.ID
			providerTx.Amount = order.Amount
		}
		if err := tx.Create(&providerTx).Error; err != nil {
			return err
		}
		virtualAccount = models.KoraVirtualAccount{
			UserID:           userID,
			AccountReference: accountReference,
			AccountName:      accountName,
			BankCode:         req.BankCode,
			Currency:         currency,
			Status:           "PENDING",
			Permanent:        req.Permanent,
			RequestPayload:   json.RawMessage(requestPayload),
			ProviderTxID:     &providerTx.ID,
		}
		if order != nil {
			virtualAccount.EscrowOrderID = &order.ID
		}
		return tx.Create(&virtualAccount).Error
	})
	if err != nil {
		return nil, err
	}

	koraResponse, err := s.koraClient.CreateVirtualAccount(ctx, kora.VirtualAccountRequest{
		AccountName:      accountName,
		AccountReference: accountReference,
		BankCode:         req.BankCode,
		Currency:         currency,
		IDNumber:         req.IDNumber,
		IDType:           req.IDType,
		Permanent:        req.Permanent,
		CustomerName:     strings.TrimSpace(user.FirstName + " " + user.LastName),
		CustomerEmail:    *user.Email,
	})
	if err != nil {
		_ = s.markProviderUnknown(providerTx.ID, err)
		virtualAccount.Status = "UNKNOWN"
		_ = s.db.Model(&virtualAccount).Update("status", "UNKNOWN").Error
		return koraVirtualAccountResponse(&virtualAccount), nil
	}

	status := normalizeProviderStatus(koraResponse.Status)
	if status == "" {
		status = "ACTIVE"
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&providerTx).Updates(map[string]interface{}{
			"status":             status,
			"external_reference": koraResponse.AccountReference,
			"response_payload":   koraResponse.Raw,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&virtualAccount).Updates(map[string]interface{}{
			"status":               status,
			"account_reference":    koraResponse.AccountReference,
			"account_name":         koraResponse.AccountName,
			"account_number":       koraResponse.AccountNumber,
			"bank_code":            firstNonEmpty(koraResponse.BankCode, req.BankCode),
			"bank_name":            koraResponse.BankName,
			"currency":             firstNonEmpty(koraResponse.Currency, currency),
			"provider_customer_id": koraResponse.ProviderCustomerID,
			"response_payload":     koraResponse.Raw,
		}).Error
	}); err != nil {
		return nil, err
	}

	virtualAccount.Status = status
	virtualAccount.AccountReference = koraResponse.AccountReference
	virtualAccount.AccountName = koraResponse.AccountName
	virtualAccount.AccountNumber = koraResponse.AccountNumber
	virtualAccount.BankCode = firstNonEmpty(koraResponse.BankCode, req.BankCode)
	virtualAccount.BankName = koraResponse.BankName
	virtualAccount.Currency = firstNonEmpty(koraResponse.Currency, currency)
	virtualAccount.ProviderCustomerID = koraResponse.ProviderCustomerID
	return koraVirtualAccountResponse(&virtualAccount), nil
}

func (s *ProviderService) ListKoraVirtualAccounts(userID uuid.UUID) ([]models.KoraVirtualAccountResponse, error) {
	var accounts []models.KoraVirtualAccount
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&accounts).Error; err != nil {
		return nil, err
	}
	responses := make([]models.KoraVirtualAccountResponse, 0, len(accounts))
	for i := range accounts {
		responses = append(responses, *koraVirtualAccountResponse(&accounts[i]))
	}
	return responses, nil
}

func (s *ProviderService) GetKoraBalances(ctx context.Context) (*models.KoraBalanceResponse, error) {
	balances, err := s.koraClient.GetBalances(ctx)
	if err != nil {
		return nil, err
	}
	response := &models.KoraBalanceResponse{
		Provider: "kora",
		Balances: make([]models.KoraBalance, 0, len(balances)),
	}
	for _, balance := range balances {
		response.Balances = append(response.Balances, models.KoraBalance{
			Currency:         balance.Currency,
			AvailableBalance: balance.AvailableBalance,
			PendingBalance:   balance.PendingBalance,
			RawAvailable:     balance.RawAvailable,
			RawPending:       balance.RawPending,
		})
	}
	return response, nil
}

func (s *ProviderService) InitiateKoraRefund(ctx context.Context, req *models.KoraRefundRequest) (*models.ProviderTransaction, error) {
	if !req.Amount.IsPositive() {
		return nil, errors.New("amount must be greater than zero")
	}
	req.Currency = strings.ToUpper(firstNonEmpty(req.Currency, "NGN"))
	reference := providerReference("KORA-REFUND")
	requestPayload, err := json.Marshal(map[string]interface{}{
		"reference":         reference,
		"payment_reference": req.PaymentReference,
		"amount_kobo":       req.Amount,
		"currency":          req.Currency,
		"reason":            req.Reason,
	})
	if err != nil {
		return nil, err
	}

	providerTx := models.ProviderTransaction{
		Provider:          "kora",
		Reference:         reference,
		ExternalReference: req.PaymentReference,
		Type:              "REFUND",
		Status:            "PENDING",
		Amount:            req.Amount,
		Currency:          req.Currency,
		RequestPayload:    json.RawMessage(requestPayload),
	}
	if err := s.db.Create(&providerTx).Error; err != nil {
		return nil, err
	}

	refundResponse, err := s.koraClient.RequestRefund(ctx, kora.RefundRequest{
		Reference:        reference,
		PaymentReference: req.PaymentReference,
		Amount:           req.Amount,
		Currency:         req.Currency,
		Reason:           req.Reason,
	})
	if err != nil {
		_ = s.markProviderUnknown(providerTx.ID, err)
		providerTx.Status = "UNKNOWN"
		return &providerTx, nil
	}

	status := normalizeProviderStatus(refundResponse.Status)
	if status == "" {
		status = "PROCESSING"
	}
	if err := s.updateProviderAfterRequest(providerTx, status, refundResponse.ExternalReference, refundResponse.Raw); err != nil {
		return nil, err
	}
	providerTx.Status = status
	providerTx.ExternalReference = firstNonEmpty(refundResponse.ExternalReference, req.PaymentReference)
	providerTx.ResponsePayload = refundResponse.Raw
	return &providerTx, nil
}

func (s *ProviderService) InitiateWalletKoraCheckout(ctx context.Context, userID uuid.UUID, req *models.KoraWalletCheckoutRequest, defaultNotificationURL string) (*models.KoraWalletCheckoutResponse, error) {
	if !req.Amount.IsPositive() {
		return nil, errors.New("amount must be greater than zero")
	}

	currency := strings.ToUpper(firstNonEmpty(req.Currency, "NGN"))
	user, err := s.userService.GetUserByID(userID.String())
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(user.Status, "ACTIVE") {
		return nil, errors.New("account is not active")
	}

	wallet, err := s.walletService.GetWalletByUserID(userID)
	if err != nil {
		return nil, err
	}
	if wallet.Status != "ACTIVE" {
		return nil, errors.New("wallet is not active")
	}
	if wallet.Currency != currency {
		return nil, fmt.Errorf("wallet currency %s cannot receive %s deposit", wallet.Currency, currency)
	}

	notificationURL := strings.TrimSpace(req.NotificationURL)
	if notificationURL == "" {
		var err error
		notificationURL, err = koraCheckoutNotificationURL(defaultNotificationURL)
		if err != nil {
			return nil, err
		}
	}

	reference := providerReference("DEP-KORA")
	narration := firstNonEmpty(req.Narration, "Direct wallet deposit")
	customerName := strings.TrimSpace(user.FirstName + " " + user.LastName)
	customerEmail := providerCustomerEmail(user)

	requestPayload, err := json.Marshal(map[string]interface{}{
		"reference":        reference,
		"user_id":          userID.String(),
		"wallet_id":        wallet.ID.String(),
		"amount_kobo":      req.Amount,
		"currency":         currency,
		"redirect_url":     req.RedirectURL,
		"notification_url": notificationURL,
		"narration":        narration,
	})
	if err != nil {
		return nil, err
	}

	var internalTx models.Transaction
	var providerTx models.ProviderTransaction
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		internalTx = models.Transaction{
			Reference:   reference,
			ToWalletID:  &wallet.ID,
			Amount:      req.Amount,
			Currency:    currency,
			Type:        "TOP_UP",
			Status:      "PENDING",
			Description: narration,
		}
		if err := tx.Create(&internalTx).Error; err != nil {
			return err
		}

		providerTx = models.ProviderTransaction{
			Provider:       "kora",
			Reference:      reference,
			UserID:         &userID,
			TransactionID:  &internalTx.ID,
			Type:           "WALLET_DEPOSIT",
			Status:         "PENDING",
			Amount:         req.Amount,
			Currency:       currency,
			RequestPayload: json.RawMessage(requestPayload),
		}
		return tx.Create(&providerTx).Error
	}); err != nil {
		return nil, err
	}

	checkoutResponse, err := s.koraClient.InitializeCheckout(ctx, kora.CheckoutRequest{
		Reference:         reference,
		Amount:            req.Amount,
		Currency:          currency,
		RedirectURL:       req.RedirectURL,
		NotificationURL:   notificationURL,
		MerchantBearsCost: false,
		CustomerName:      customerName,
		CustomerEmail:     customerEmail,
		CustomerPhone:     user.Phone,
		Narration:         narration,
		Metadata: map[string]interface{}{
			"flow":      "wallet_deposit",
			"user_id":   userID.String(),
			"wallet_id": wallet.ID.String(),
		},
	})
	if err != nil {
		_ = s.markProviderUnknown(providerTx.ID, err)
		_ = s.db.Model(&models.Transaction{}).Where("id = ?", internalTx.ID).Update("status", "FAILED").Error
		return nil, err
	}
	if strings.TrimSpace(checkoutResponse.CheckoutURL) == "" {
		err := errors.New("kora checkout url was not returned")
		_ = s.markProviderUnknown(providerTx.ID, err)
		_ = s.db.Model(&models.Transaction{}).Where("id = ?", internalTx.ID).Update("status", "FAILED").Error
		return nil, err
	}

	status := normalizeProviderStatus(checkoutResponse.Status)
	if status == "" || status == "SUCCESS" {
		status = "PROCESSING"
	}
	if err := s.updateProviderAfterRequest(providerTx, status, checkoutResponse.Reference, checkoutResponse.Raw); err != nil {
		return nil, err
	}

	return &models.KoraWalletCheckoutResponse{
		Success:     true,
		CheckoutURL: checkoutResponse.CheckoutURL,
		Reference:   reference,
	}, nil
}

func (s *ProviderService) InitiateEscrowKoraCheckout(ctx context.Context, userID uuid.UUID, escrowReference string, req *models.KoraEscrowCheckoutRequest, defaultNotificationURL string) (*models.EscrowCheckoutResponse, error) {
	var order models.EscrowOrder
	if err := s.db.Where("reference = ?", escrowReference).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("escrow order not found")
		}
		return nil, err
	}
	if order.Status != "CREATED" {
		return nil, fmt.Errorf("escrow order cannot be checked out from state %s", order.Status)
	}

	initiator, err := s.userService.GetUserByID(userID.String())
	if err != nil {
		return nil, err
	}
	if !isUserKYCVerified(initiator) {
		return nil, errors.New("complete Kora KYC before starting escrow checkout")
	}

	initiatedBySeller := order.SellerID == userID
	if err := s.ensureNoConflictingEscrowPaymentFlow(order.ID, userID, initiatedBySeller); err != nil {
		return nil, err
	}

	notificationURL := strings.TrimSpace(req.NotificationURL)
	if notificationURL == "" {
		var err error
		notificationURL, err = koraCheckoutNotificationURL(defaultNotificationURL)
		if err != nil {
			return nil, err
		}
	}

	checkoutSource := "buyer_authenticated"
	payerType := "registered_user"
	customerName := strings.TrimSpace(initiator.FirstName + " " + initiator.LastName)
	customerEmail := ""
	if initiator.Email != nil {
		customerEmail = strings.TrimSpace(*initiator.Email)
	}
	customerPhone := initiator.Phone
	var buyerUserID *uuid.UUID
	var buyerWalletID *uuid.UUID

	if initiatedBySeller {
		checkoutSource = "seller_share_link"
		payerType = "external"
		if order.BuyerID != nil {
			return nil, errors.New("escrow order already has a committed buyer")
		}
		customerName = strings.TrimSpace(req.BuyerName)
		customerEmail = strings.TrimSpace(req.BuyerEmail)
		customerPhone = strings.TrimSpace(req.BuyerPhone)
		if customerEmail == "" {
			return nil, errors.New("buyer_email is required when seller generates a Kora checkout link")
		}
	} else {
		if order.BuyerID != nil && *order.BuyerID != userID {
			return nil, errors.New("another buyer has already started checkout for this escrow order")
		}
		if customerEmail == "" {
			return nil, errors.New("buyer email is required to start Kora checkout")
		}

		var buyerWallet models.Wallet
		if err := s.db.Where("owner_type = ? AND user_id = ?", "USER", userID).First(&buyerWallet).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("buyer wallet not found")
			}
			return nil, err
		}
		if buyerWallet.Currency != order.Currency {
			return nil, fmt.Errorf("buyer wallet currency %s does not match order currency %s", buyerWallet.Currency, order.Currency)
		}
		buyerUserID = &userID
		buyerWalletID = &buyerWallet.ID
	}

	koraCustomerName := firstNonEmpty(customerName, "ESCRA Buyer")
	providerReference := providerReference("ESCROW")
	requestPayload, err := json.Marshal(map[string]interface{}{
		"escrow_reference":     order.Reference,
		"checkout_source":      checkoutSource,
		"payer_type":           payerType,
		"initiated_by_user_id": userID.String(),
		"seller_id":            order.SellerID.String(),
		"buyer_id":             uuidStringOrEmpty(buyerUserID),
		"buyer_email":          customerEmail,
		"buyer_name":           customerName,
		"buyer_phone":          customerPhone,
		"notification_url":     notificationURL,
		"redirect_url":         req.RedirectURL,
		"narration":            req.Narration,
		"merchant_bears_cost":  req.MerchantBearsCost,
		"default_channel":      req.DefaultChannel,
		"channels":             req.Channels,
		"currency":             order.Currency,
		"amount_kobo":          order.Amount,
	})
	if err != nil {
		return nil, err
	}

	providerTx := models.ProviderTransaction{
		Provider:          "kora",
		Reference:         providerReference,
		ExternalReference: "",
		UserID:            buyerUserID,
		EscrowOrderID:     &order.ID,
		Type:              "ESCROW_PAYIN",
		Status:            "PENDING",
		Amount:            order.Amount,
		Currency:          order.Currency,
		RequestPayload:    json.RawMessage(requestPayload),
	}
	if err := s.db.Create(&providerTx).Error; err != nil {
		return nil, err
	}

	checkoutResponse, err := s.koraClient.InitializeCheckout(ctx, kora.CheckoutRequest{
		Reference:         providerReference,
		Amount:            order.Amount,
		Currency:          order.Currency,
		RedirectURL:       req.RedirectURL,
		NotificationURL:   notificationURL,
		DefaultChannel:    req.DefaultChannel,
		Channels:          req.Channels,
		MerchantBearsCost: req.MerchantBearsCost,
		CustomerName:      koraCustomerName,
		CustomerEmail:     customerEmail,
		CustomerPhone:     customerPhone,
		Narration:         firstNonEmpty(req.Narration, fmt.Sprintf("Escrow payment for %s", order.Reference)),
		Metadata: map[string]interface{}{
			"escrow_reference":     order.Reference,
			"checkout_source":      checkoutSource,
			"initiated_by_user_id": userID.String(),
			"seller_id":            order.SellerID.String(),
			"buyer_id":             uuidStringOrEmpty(buyerUserID),
			"buyer_email":          customerEmail,
			"buyer_phone":          customerPhone,
		},
	})
	if err != nil {
		_ = s.markProviderUnknown(providerTx.ID, err)
		providerTx.Status = "UNKNOWN"
		orderResponse, buildErr := s.buildEscrowCheckoutOrderResponse(&order)
		if buildErr != nil {
			return nil, buildErr
		}
		return &models.EscrowCheckoutResponse{
			Order:               *orderResponse,
			Provider:            "kora",
			ProviderReference:   providerTx.Reference,
			CheckoutReference:   providerTx.Reference,
			CheckoutStatus:      providerTx.Status,
			RedirectURL:         req.RedirectURL,
			NotificationURL:     notificationURL,
			MerchantBearsCost:   req.MerchantBearsCost,
			DefaultChannel:      req.DefaultChannel,
			Channels:            req.Channels,
			ProviderTransaction: providerTransactionResponseDTO(&providerTx),
		}, nil
	}

	status := normalizeProviderStatus(checkoutResponse.Status)
	if status == "" {
		status = "PROCESSING"
	}
	if err := s.updateProviderAfterRequest(providerTx, status, checkoutResponse.Reference, checkoutResponse.Raw); err != nil {
		return nil, err
	}
	providerTx.Status = status
	providerTx.ExternalReference = checkoutResponse.Reference
	providerTx.ResponsePayload = checkoutResponse.Raw

	orderUpdates := map[string]interface{}{
		"buyer_name":  customerName,
		"buyer_email": customerEmail,
		"buyer_phone": customerPhone,
	}
	if !initiatedBySeller {
		orderUpdates["buyer_id"] = *buyerUserID
		orderUpdates["buyer_wallet_id"] = *buyerWalletID
	}
	if err := s.db.Model(&order).Updates(orderUpdates).Error; err != nil {
		return nil, err
	}
	if !initiatedBySeller {
		order.BuyerID = buyerUserID
		order.BuyerWalletID = buyerWalletID
	}
	order.BuyerName = customerName
	order.BuyerEmail = customerEmail
	order.BuyerPhone = customerPhone

	orderResponse, err := s.buildEscrowCheckoutOrderResponse(&order)
	if err != nil {
		return nil, err
	}
	return &models.EscrowCheckoutResponse{
		Order:               *orderResponse,
		Provider:            "kora",
		ProviderReference:   providerTx.Reference,
		CheckoutURL:         checkoutResponse.CheckoutURL,
		CheckoutReference:   checkoutResponse.Reference,
		CheckoutStatus:      providerTx.Status,
		RedirectURL:         req.RedirectURL,
		NotificationURL:     notificationURL,
		MerchantBearsCost:   req.MerchantBearsCost,
		DefaultChannel:      req.DefaultChannel,
		Channels:            req.Channels,
		ProviderTransaction: providerTransactionResponseDTO(&providerTx),
	}, nil
}

func (s *ProviderService) InitiateKoraBankPayout(ctx context.Context, userID uuid.UUID, req *models.KoraBankPayoutRequest) (*models.ProviderTransaction, error) {
	if !req.Amount.IsPositive() {
		return nil, errors.New("amount must be greater than zero")
	}
	if req.Currency == "" {
		req.Currency = "NGN"
	}
	req.Currency = strings.ToUpper(req.Currency)

	var user models.User
	var providerTx models.ProviderTransaction
	var internalTx models.Transaction
	var senderWallet models.Wallet

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("user not found")
			}
			return err
		}
		if err := utils.CheckPassword(user.PinHash, req.Pin); err != nil {
			return errors.New("invalid PIN")
		}
		if !isUserKYCVerified(&user) {
			return errors.New("complete Kora KYC before requesting payout")
		}

		customerEmail := req.CustomerEmail
		if customerEmail == "" && user.Email != nil {
			customerEmail = *user.Email
		}
		if customerEmail == "" {
			return errors.New("customer email is required for Kora payout")
		}

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner_type = ? AND user_id = ?", "USER", userID).
			First(&senderWallet).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("sender wallet not found")
			}
			return err
		}
		if senderWallet.Status != "ACTIVE" {
			return errors.New("sender wallet is not active")
		}
		if senderWallet.Currency != req.Currency {
			return fmt.Errorf("wallet currency %s cannot fund %s payout", senderWallet.Currency, req.Currency)
		}
		if senderWallet.Balance < req.Amount {
			return errors.New("insufficient funds")
		}

		if err := s.transactionLimitService.CheckAndRecordTransaction(tx, userID, req.Amount); err != nil {
			return err
		}

		reference := providerReference("KORA")
		internalTx = models.Transaction{
			Reference:    reference,
			FromWalletID: &senderWallet.ID,
			Amount:       req.Amount,
			Currency:     req.Currency,
			Type:         "WITHDRAWAL",
			Status:       "PENDING",
			Description:  firstNonEmpty(req.Narration, "Bank payout"),
		}
		if err := tx.Create(&internalTx).Error; err != nil {
			return err
		}

		newBalance := senderWallet.Balance - req.Amount
		if err := tx.Model(&senderWallet).Update("balance", newBalance).Error; err != nil {
			return err
		}
		ledgerEntry := models.LedgerEntry{
			TransactionID: internalTx.ID,
			WalletID:      senderWallet.ID,
			Debit:         req.Amount,
			Credit:        money.Zero,
			BalanceAfter:  newBalance,
		}
		if err := tx.Create(&ledgerEntry).Error; err != nil {
			return err
		}

		requestPayload, err := json.Marshal(map[string]interface{}{
			"reference":      reference,
			"bank_code":      req.BankCode,
			"account_number": req.AccountNumber,
			"account_name":   req.AccountName,
			"customer_email": customerEmail,
			"amount_kobo":    req.Amount,
			"currency":       req.Currency,
			"narration":      req.Narration,
		})
		if err != nil {
			return err
		}

		providerTx = models.ProviderTransaction{
			Provider:       "kora",
			Reference:      reference,
			UserID:         &userID,
			TransactionID:  &internalTx.ID,
			Type:           "BANK_PAYOUT",
			Status:         "PENDING",
			Amount:         req.Amount,
			Currency:       req.Currency,
			RequestPayload: json.RawMessage(requestPayload),
		}
		return tx.Create(&providerTx).Error
	})
	if err != nil {
		return nil, err
	}

	customerEmail := req.CustomerEmail
	if customerEmail == "" && user.Email != nil {
		customerEmail = *user.Email
	}
	accountName := req.AccountName
	if accountName == "" {
		accountName = strings.TrimSpace(user.FirstName + " " + user.LastName)
	}

	payoutResponse, err := s.koraClient.RequestPayout(ctx, kora.PayoutRequest{
		Reference:   providerTx.Reference,
		Amount:      req.Amount,
		Currency:    req.Currency,
		Narration:   req.Narration,
		BankCode:    req.BankCode,
		AccountName: accountName,
		AccountNo:   req.AccountNumber,
		Email:       customerEmail,
	})
	if err != nil {
		_ = s.markProviderUnknown(providerTx.ID, err)
		providerTx.Status = "UNKNOWN"
		return &providerTx, nil
	}

	status := normalizeProviderStatus(payoutResponse.Status)
	if status == "" {
		status = "PROCESSING"
	}
	if err := s.updateProviderAfterRequest(providerTx, status, payoutResponse.ExternalReference, payoutResponse.Raw); err != nil {
		return nil, err
	}
	providerTx.Status = status
	providerTx.ExternalReference = payoutResponse.ExternalReference
	providerTx.ResponsePayload = payoutResponse.Raw
	return &providerTx, nil
}

func (s *ProviderService) InitiateQuidaxWithdrawal(ctx context.Context, userID uuid.UUID, req *models.QuidaxCryptoWithdrawalRequest) (*models.ProviderTransaction, error) {
	if strings.TrimSpace(req.Amount) == "" {
		return nil, errors.New("amount is required")
	}
	req.Currency = strings.ToLower(strings.TrimSpace(req.Currency))
	if req.Currency == "" {
		return nil, errors.New("currency is required")
	}

	user, err := s.userService.GetUserByID(userID.String())
	if err != nil {
		return nil, err
	}
	if err := utils.CheckPassword(user.PinHash, req.Pin); err != nil {
		return nil, errors.New("invalid PIN")
	}

	reference := providerReference("QDX")
	requestPayload, err := json.Marshal(map[string]interface{}{
		"reference":        reference,
		"currency":         req.Currency,
		"amount":           req.Amount,
		"fund_uid":         req.FundUID,
		"fund_uid2":        req.FundUID2,
		"network":          req.Network,
		"transaction_note": req.TransactionNote,
		"narration":        req.Narration,
		"provider_user_id": req.ProviderUserID,
	})
	if err != nil {
		return nil, err
	}

	providerTx := models.ProviderTransaction{
		Provider:       "quidax",
		Reference:      reference,
		UserID:         &userID,
		Type:           "CRYPTO_WITHDRAWAL",
		Status:         "PENDING",
		AmountText:     req.Amount,
		Currency:       req.Currency,
		RequestPayload: json.RawMessage(requestPayload),
	}
	if err := s.db.Create(&providerTx).Error; err != nil {
		return nil, err
	}

	withdrawResponse, err := s.quidaxClient.Withdraw(ctx, quidax.WithdrawRequest{
		UserID:          req.ProviderUserID,
		Currency:        req.Currency,
		Amount:          req.Amount,
		FundUID:         req.FundUID,
		FundUID2:        req.FundUID2,
		TransactionNote: req.TransactionNote,
		Narration:       req.Narration,
		Network:         req.Network,
		Reference:       reference,
	})
	if err != nil {
		_ = s.markProviderUnknown(providerTx.ID, err)
		providerTx.Status = "UNKNOWN"
		return &providerTx, nil
	}

	status := normalizeProviderStatus(withdrawResponse.Status)
	if status == "" {
		status = "PROCESSING"
	}
	if err := s.updateProviderAfterRequest(providerTx, status, withdrawResponse.ExternalReference, withdrawResponse.Raw); err != nil {
		return nil, err
	}
	providerTx.Status = status
	providerTx.ExternalReference = withdrawResponse.ExternalReference
	providerTx.ResponsePayload = withdrawResponse.Raw
	return &providerTx, nil
}

func (s *ProviderService) updateProviderAfterRequest(providerTx models.ProviderTransaction, status, externalReference string, response json.RawMessage) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.ProviderTransaction{}).
			Where("id = ?", providerTx.ID).
			Updates(map[string]interface{}{
				"status":             status,
				"external_reference": externalReference,
				"response_payload":   response,
			}).Error; err != nil {
			return err
		}

		if providerTx.TransactionID == nil || (status != "SUCCESS" && status != "FAILED") {
			return nil
		}

		return updateInternalTransactionFromProvider(tx, providerTx, status)
	})
}

func (s *ProviderService) ensureNoConflictingEscrowPaymentFlow(orderID, actorID uuid.UUID, actorIsSeller bool) error {
	var providerTxs []models.ProviderTransaction
	if err := s.db.Where(
		"escrow_order_id = ? AND type IN ? AND status IN ?",
		orderID,
		[]string{"ESCROW_PAYIN", "ESCROW_VIRTUAL_ACCOUNT"},
		[]string{"PENDING", "PROCESSING", "SUCCESS"},
	).Find(&providerTxs).Error; err != nil {
		return err
	}

	for _, providerTx := range providerTxs {
		if actorIsSeller {
			return errors.New("escrow order already has an active payment flow")
		}
		if providerTx.UserID == nil || *providerTx.UserID != actorID {
			return errors.New("another buyer has already started checkout for this escrow order")
		}
	}
	return nil
}

func koraCheckoutNotificationURL(publicBaseURL string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if baseURL == "" {
		return "", errors.New("server is missing PUBLIC_BASE_URL or notification_url")
	}
	return baseURL + "/api/v1/webhooks/kora", nil
}

func (s *ProviderService) buildEscrowCheckoutOrderResponse(order *models.EscrowOrder) (*models.EscrowOrderResponse, error) {
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
		BuyerName:            order.BuyerName,
		BuyerEmail:           order.BuyerEmail,
		BuyerPhone:           order.BuyerPhone,
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

func (s *ProviderService) markProviderUnknown(id uuid.UUID, providerErr error) error {
	payload, _ := json.Marshal(map[string]string{"error": providerErr.Error()})
	return s.db.Model(&models.ProviderTransaction{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":           "UNKNOWN",
			"response_payload": json.RawMessage(payload),
		}).Error
}

func providerReference(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, uuid.New().String())
}

func normalizeProviderStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "successful", "succeeded", "completed", "complete", "done":
		return "SUCCESS"
	case "failed", "failure", "rejected", "cancelled", "canceled":
		return "FAILED"
	case "processing", "pending", "queued", "accepted", "true":
		return "PROCESSING"
	case "":
		return ""
	default:
		return strings.ToUpper(status)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func uuidStringOrEmpty(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func providerTransactionResponseDTO(tx *models.ProviderTransaction) models.ProviderTransactionResponse {
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

func koraVirtualAccountResponse(account *models.KoraVirtualAccount) *models.KoraVirtualAccountResponse {
	return &models.KoraVirtualAccountResponse{
		ID:                 account.ID.String(),
		AccountReference:   account.AccountReference,
		AccountName:        account.AccountName,
		AccountNumber:      account.AccountNumber,
		BankCode:           account.BankCode,
		BankName:           account.BankName,
		Currency:           account.Currency,
		Status:             account.Status,
		Permanent:          account.Permanent,
		ProviderCustomerID: account.ProviderCustomerID,
		CreatedAt:          account.CreatedAt,
	}
}

func isUserKYCVerified(user *models.User) bool {
	return strings.EqualFold(user.KYCStatus, "VERIFIED")
}

func providerCustomerEmail(user *models.User) string {
	if user.Email != nil && strings.TrimSpace(*user.Email) != "" {
		return strings.TrimSpace(*user.Email)
	}
	return strings.ToLower(user.ID.String()) + "@escra.local"
}

func last4(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 4 {
		return value
	}
	return value[len(value)-4:]
}

func timeNowUTC() time.Time {
	return time.Now().UTC()
}
