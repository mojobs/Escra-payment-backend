//go:build cgo
// +build cgo

package services

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/pkg/money"
	"gorm.io/gorm"
)

func TestBuyerInitiatedEscrowKoraCheckoutKeepsExistingBehavior(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)
	walletService := NewWalletService(db)
	limitService := NewTransactionLimitService(db)
	fakeKora := &fakeWalletCheckoutKoraClient{}
	providerService := NewProviderService(db, userService, walletService, limitService, fakeKora, nil)

	seller := createTestUserWithWallet(t, db, userService, walletService, "08000000941", "1234", money.Zero)
	buyer := createTestUserWithWallet(t, db, userService, walletService, "08000000942", "1234", money.Zero)
	markUserVerifiedWithEmail(t, db, buyer, "buyer-checkout@example.com")

	escrowService := NewEscrowService(db, userService, walletService, limitService)
	order, err := escrowService.CreateOrder(seller.ID, &models.CreateEscrowOrderRequest{
		Title:    "Sneakers",
		Amount:   money.FromMinorUnits(150000),
		Currency: "NGN",
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	response, err := providerService.InitiateEscrowKoraCheckout(context.Background(), buyer.ID, order.Reference, &models.KoraEscrowCheckoutRequest{
		RedirectURL: "https://escra.app/pay/return",
	}, "https://api.escra.test")
	if err != nil {
		t.Fatalf("initiate buyer checkout: %v", err)
	}

	if response.CheckoutURL == "" {
		t.Fatalf("expected checkout URL")
	}
	if response.Order.BuyerID != buyer.ID.String() {
		t.Fatalf("buyer id = %q", response.Order.BuyerID)
	}
	if fakeKora.checkoutRequest.CustomerEmail != "buyer-checkout@example.com" {
		t.Fatalf("customer email = %q", fakeKora.checkoutRequest.CustomerEmail)
	}
	if fakeKora.checkoutRequest.Metadata["checkout_source"] != "buyer_authenticated" {
		t.Fatalf("checkout source metadata = %v", fakeKora.checkoutRequest.Metadata["checkout_source"])
	}
	if fakeKora.checkoutRequest.NotificationURL != "https://api.escra.test/api/v1/webhooks/kora" {
		t.Fatalf("notification url = %q", fakeKora.checkoutRequest.NotificationURL)
	}

	var providerTx models.ProviderTransaction
	if err := db.Where("reference = ?", response.ProviderReference).First(&providerTx).Error; err != nil {
		t.Fatalf("find provider tx: %v", err)
	}
	if providerTx.UserID == nil || *providerTx.UserID != buyer.ID {
		t.Fatalf("provider tx user id = %v", providerTx.UserID)
	}
	payload := providerPayload(t, providerTx)
	if payload["initiated_by_user_id"] != buyer.ID.String() {
		t.Fatalf("initiated_by_user_id = %v", payload["initiated_by_user_id"])
	}
	if payload["buyer_id"] != buyer.ID.String() {
		t.Fatalf("buyer_id = %v", payload["buyer_id"])
	}
}

func TestSellerCanGenerateEscrowKoraCheckoutLinkWithoutBecomingBuyer(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)
	walletService := NewWalletService(db)
	limitService := NewTransactionLimitService(db)
	fakeKora := &fakeWalletCheckoutKoraClient{}
	providerService := NewProviderService(db, userService, walletService, limitService, fakeKora, nil)

	seller := createTestUserWithWallet(t, db, userService, walletService, "08000000943", "1234", money.Zero)
	markUserVerifiedWithEmail(t, db, seller, "seller@example.com")

	escrowService := NewEscrowService(db, userService, walletService, limitService)
	order, err := escrowService.CreateOrder(seller.ID, &models.CreateEscrowOrderRequest{
		Title:    "Handmade bag",
		Amount:   money.FromMinorUnits(220000),
		Currency: "NGN",
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	response, err := providerService.InitiateEscrowKoraCheckout(context.Background(), seller.ID, order.Reference, &models.KoraEscrowCheckoutRequest{
		RedirectURL: "https://escra.app/pay/return",
		BuyerName:   "Ada Buyer",
		BuyerEmail:  "ada@example.com",
		BuyerPhone:  "08030000000",
	}, "https://api.escra.test")
	if err != nil {
		t.Fatalf("initiate seller checkout link: %v", err)
	}

	if response.CheckoutURL == "" {
		t.Fatalf("expected checkout URL")
	}
	if response.Order.BuyerID != "" {
		t.Fatalf("seller-generated checkout should not set buyer id, got %q", response.Order.BuyerID)
	}
	if response.Order.BuyerEmail != "ada@example.com" {
		t.Fatalf("buyer email = %q", response.Order.BuyerEmail)
	}
	if fakeKora.checkoutRequest.CustomerEmail != "ada@example.com" {
		t.Fatalf("customer email = %q", fakeKora.checkoutRequest.CustomerEmail)
	}
	if fakeKora.checkoutRequest.Metadata["checkout_source"] != "seller_share_link" {
		t.Fatalf("checkout source metadata = %v", fakeKora.checkoutRequest.Metadata["checkout_source"])
	}

	var providerTx models.ProviderTransaction
	if err := db.Where("reference = ?", response.ProviderReference).First(&providerTx).Error; err != nil {
		t.Fatalf("find provider tx: %v", err)
	}
	if providerTx.UserID != nil {
		t.Fatalf("seller-generated provider tx should not have user_id, got %v", providerTx.UserID)
	}
	payload := providerPayload(t, providerTx)
	if payload["initiated_by_user_id"] != seller.ID.String() {
		t.Fatalf("initiated_by_user_id = %v", payload["initiated_by_user_id"])
	}
	if payload["buyer_id"] != "" {
		t.Fatalf("buyer_id should be empty, got %v", payload["buyer_id"])
	}
	if payload["buyer_email"] != "ada@example.com" {
		t.Fatalf("buyer_email = %v", payload["buyer_email"])
	}

	var orderRow models.EscrowOrder
	if err := db.Where("id = ?", order.ID).First(&orderRow).Error; err != nil {
		t.Fatalf("find order: %v", err)
	}
	if orderRow.BuyerID != nil || orderRow.BuyerWalletID != nil {
		t.Fatalf("seller link should not set buyer pointers: buyer=%v wallet=%v", orderRow.BuyerID, orderRow.BuyerWalletID)
	}
}

func TestEscrowKoraCheckoutUsesRequestNotificationURLOverride(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)
	walletService := NewWalletService(db)
	limitService := NewTransactionLimitService(db)
	fakeKora := &fakeWalletCheckoutKoraClient{}
	providerService := NewProviderService(db, userService, walletService, limitService, fakeKora, nil)

	seller := createTestUserWithWallet(t, db, userService, walletService, "08000000945", "1234", money.Zero)
	buyer := createTestUserWithWallet(t, db, userService, walletService, "08000000946", "1234", money.Zero)
	markUserVerifiedWithEmail(t, db, buyer, "override-buyer@example.com")

	escrowService := NewEscrowService(db, userService, walletService, limitService)
	order, err := escrowService.CreateOrder(seller.ID, &models.CreateEscrowOrderRequest{
		Title:    "Override checkout",
		Amount:   money.FromMinorUnits(120000),
		Currency: "NGN",
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	_, err = providerService.InitiateEscrowKoraCheckout(context.Background(), buyer.ID, order.Reference, &models.KoraEscrowCheckoutRequest{
		NotificationURL: "https://internal.example.test/webhooks/kora",
	}, "")
	if err != nil {
		t.Fatalf("initiate checkout with notification override: %v", err)
	}
	if fakeKora.checkoutRequest.NotificationURL != "https://internal.example.test/webhooks/kora" {
		t.Fatalf("notification url = %q", fakeKora.checkoutRequest.NotificationURL)
	}
}

func TestEscrowKoraCheckoutMissingNotificationURLConfigReturnsClearError(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)
	walletService := NewWalletService(db)
	limitService := NewTransactionLimitService(db)
	providerService := NewProviderService(db, userService, walletService, limitService, &fakeWalletCheckoutKoraClient{}, nil)

	seller := createTestUserWithWallet(t, db, userService, walletService, "08000000947", "1234", money.Zero)
	buyer := createTestUserWithWallet(t, db, userService, walletService, "08000000948", "1234", money.Zero)
	markUserVerifiedWithEmail(t, db, buyer, "missing-config-buyer@example.com")

	escrowService := NewEscrowService(db, userService, walletService, limitService)
	order, err := escrowService.CreateOrder(seller.ID, &models.CreateEscrowOrderRequest{
		Title:    "Missing config checkout",
		Amount:   money.FromMinorUnits(130000),
		Currency: "NGN",
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	_, err = providerService.InitiateEscrowKoraCheckout(context.Background(), buyer.ID, order.Reference, &models.KoraEscrowCheckoutRequest{
		MerchantBearsCost: false,
	}, "")
	if err == nil {
		t.Fatalf("expected missing PUBLIC_BASE_URL error")
	}
	if err.Error() != "server is missing PUBLIC_BASE_URL or notification_url" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSellerEscrowKoraCheckoutRequiresBuyerEmail(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)
	walletService := NewWalletService(db)
	limitService := NewTransactionLimitService(db)
	providerService := NewProviderService(db, userService, walletService, limitService, &fakeWalletCheckoutKoraClient{}, nil)

	seller := createTestUserWithWallet(t, db, userService, walletService, "08000000944", "1234", money.Zero)
	markUserVerifiedWithEmail(t, db, seller, "seller2@example.com")

	escrowService := NewEscrowService(db, userService, walletService, limitService)
	order, err := escrowService.CreateOrder(seller.ID, &models.CreateEscrowOrderRequest{
		Title:    "Wristwatch",
		Amount:   money.FromMinorUnits(90000),
		Currency: "NGN",
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	_, err = providerService.InitiateEscrowKoraCheckout(context.Background(), seller.ID, order.Reference, &models.KoraEscrowCheckoutRequest{
		BuyerName: "No Email Buyer",
	}, "https://api.escra.test")
	if err == nil {
		t.Fatalf("expected buyer_email validation error")
	}
	if !strings.Contains(err.Error(), "buyer_email is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func markUserVerifiedWithEmail(t *testing.T, db *gorm.DB, user *models.User, email string) {
	t.Helper()
	if err := db.Model(user).Updates(map[string]interface{}{
		"email":      email,
		"kyc_status": "VERIFIED",
	}).Error; err != nil {
		t.Fatalf("mark user verified: %v", err)
	}
}

func providerPayload(t *testing.T, providerTx models.ProviderTransaction) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(providerTx.RequestPayload, &payload); err != nil {
		t.Fatalf("unmarshal provider request payload: %v", err)
	}
	return payload
}
