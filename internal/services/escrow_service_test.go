//go:build cgo
// +build cgo

package services

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/pkg/money"
)

func TestEscrowLifecycleFundAndRelease(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)
	walletService := NewWalletService(db)
	limitService := NewTransactionLimitService(db)
	escrowService := NewEscrowService(db, userService, walletService, limitService)

	seller := createTestUserWithWallet(t, db, userService, walletService, "08000000001", "1111", money.Zero)
	buyer := createTestUserWithWallet(t, db, userService, walletService, "08000000002", "2222", money.FromMinorUnits(500000))

	order, err := escrowService.CreateOrder(seller.ID, &models.CreateEscrowOrderRequest{
		Title:        "Ankara gown",
		Description:  "Custom order",
		Amount:       money.FromMinorUnits(150000),
		Currency:     "NGN",
		SalesChannel: "WHATSAPP",
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	funded, err := escrowService.FundOrder(buyer.ID, order.Reference, &models.FundEscrowOrderRequest{Pin: "2222"})
	if err != nil {
		t.Fatalf("fund order: %v", err)
	}
	if funded.Order.Status != "FUNDED" {
		t.Fatalf("got funded status %q", funded.Order.Status)
	}
	if funded.DeliveryCode == "" {
		t.Fatalf("expected delivery code")
	}

	if _, err := escrowService.MarkShipped(seller.ID, order.Reference, &models.ShipEscrowOrderRequest{
		TrackingReference: "TRK-123",
	}); err != nil {
		t.Fatalf("mark shipped: %v", err)
	}
	if _, err := escrowService.MarkDelivered(seller.ID, order.Reference, &models.MarkDeliveredEscrowOrderRequest{}); err != nil {
		t.Fatalf("mark delivered: %v", err)
	}
	released, err := escrowService.ConfirmDelivery(buyer.ID, order.Reference, &models.ConfirmEscrowDeliveryRequest{
		DeliveryCode: funded.DeliveryCode,
	})
	if err != nil {
		t.Fatalf("confirm delivery: %v", err)
	}
	if released.Status != "RELEASED" {
		t.Fatalf("got release status %q", released.Status)
	}

	buyerWallet, err := walletService.GetWalletByUserID(buyer.ID)
	if err != nil {
		t.Fatalf("get buyer wallet: %v", err)
	}
	sellerWallet, err := walletService.GetWalletByUserID(seller.ID)
	if err != nil {
		t.Fatalf("get seller wallet: %v", err)
	}
	escrowWallet, err := walletService.GetWalletByReference("SYSTEM", "ESCROW_NGN")
	if err != nil {
		t.Fatalf("get escrow wallet: %v", err)
	}

	if buyerWallet.Balance != money.FromMinorUnits(350000) {
		t.Fatalf("got buyer balance %d", buyerWallet.Balance)
	}
	if sellerWallet.Balance != money.FromMinorUnits(150000) {
		t.Fatalf("got seller balance %d", sellerWallet.Balance)
	}
	if escrowWallet.Balance != money.Zero {
		t.Fatalf("got escrow balance %d", escrowWallet.Balance)
	}
}

func TestEscrowOrderResponsesIncludePublicShareURL(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)
	walletService := NewWalletService(db)
	limitService := NewTransactionLimitService(db)
	escrowService := NewEscrowService(db, userService, walletService, limitService, "https://escra-payment-backend.onrender.com")

	seller := createTestUserWithWallet(t, db, userService, walletService, "08000000021", "1111", money.Zero)
	order, err := escrowService.CreateOrder(seller.ID, &models.CreateEscrowOrderRequest{
		Title:        "Public preview watch",
		Description:  "Safe public description",
		Amount:       money.FromMinorUnits(250000),
		Currency:     "NGN",
		SalesChannel: "INSTAGRAM",
		DeliveryMode: "PHYSICAL",
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	expectedURL := "https://escra-payment-backend.onrender.com/api/v1/public/escrows/orders/" + order.Reference
	if order.PublicURL != expectedURL {
		t.Fatalf("create response public url = %q", order.PublicURL)
	}

	privateOrder, err := escrowService.GetOrder(seller.ID, order.Reference)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if privateOrder.PublicURL != expectedURL {
		t.Fatalf("get response public url = %q", privateOrder.PublicURL)
	}

	orders, err := escrowService.ListOrders(seller.ID)
	if err != nil {
		t.Fatalf("list orders: %v", err)
	}
	if len(orders) != 1 || orders[0].PublicURL != expectedURL {
		t.Fatalf("list response public url = %+v", orders)
	}

	publicOrder, err := escrowService.GetPublicOrder(order.Reference)
	if err != nil {
		t.Fatalf("get public order: %v", err)
	}
	if publicOrder.PublicURL != expectedURL {
		t.Fatalf("public preview url = %q", publicOrder.PublicURL)
	}
	if publicOrder.Title != "Public preview watch" || publicOrder.Amount != money.FromMinorUnits(250000) || publicOrder.Status != "CREATED" {
		t.Fatalf("unexpected public preview: %+v", publicOrder)
	}
	if publicOrder.SellerName == "" {
		t.Fatalf("expected seller display name")
	}
}

func TestEscrowLifecycleReleaseWithPin(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)
	walletService := NewWalletService(db)
	limitService := NewTransactionLimitService(db)
	escrowService := NewEscrowService(db, userService, walletService, limitService)

	seller := createTestUserWithWallet(t, db, userService, walletService, "08000000011", "1111", money.Zero)
	buyer := createTestUserWithWallet(t, db, userService, walletService, "08000000012", "2222", money.FromMinorUnits(300000))

	order, err := escrowService.CreateOrder(seller.ID, &models.CreateEscrowOrderRequest{
		Title:    "Sneaker preorder",
		Amount:   money.FromMinorUnits(120000),
		Currency: "NGN",
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	if _, err := escrowService.FundOrder(buyer.ID, order.Reference, &models.FundEscrowOrderRequest{Pin: "2222"}); err != nil {
		t.Fatalf("fund order: %v", err)
	}
	if _, err := escrowService.MarkShipped(seller.ID, order.Reference, &models.ShipEscrowOrderRequest{}); err != nil {
		t.Fatalf("mark shipped: %v", err)
	}
	if _, err := escrowService.MarkDelivered(seller.ID, order.Reference, &models.MarkDeliveredEscrowOrderRequest{}); err != nil {
		t.Fatalf("mark delivered: %v", err)
	}

	released, err := escrowService.ConfirmDelivery(buyer.ID, order.Reference, &models.ConfirmEscrowDeliveryRequest{
		Pin: "2222",
	})
	if err != nil {
		t.Fatalf("confirm with pin: %v", err)
	}
	if released.Status != "RELEASED" {
		t.Fatalf("got release status %q", released.Status)
	}
}

func TestEscrowDisputeAndRefund(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)
	walletService := NewWalletService(db)
	limitService := NewTransactionLimitService(db)
	escrowService := NewEscrowService(db, userService, walletService, limitService)

	seller := createTestUserWithWallet(t, db, userService, walletService, "08000000003", "3333", money.Zero)
	buyer := createTestUserWithWallet(t, db, userService, walletService, "08000000004", "4444", money.FromMinorUnits(250000))

	order, err := escrowService.CreateOrder(seller.ID, &models.CreateEscrowOrderRequest{
		Title:    "Phone case",
		Amount:   money.FromMinorUnits(100000),
		Currency: "NGN",
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if _, err := escrowService.FundOrder(buyer.ID, order.Reference, &models.FundEscrowOrderRequest{Pin: "4444"}); err != nil {
		t.Fatalf("fund order: %v", err)
	}

	disputed, err := escrowService.OpenDispute(buyer.ID, order.Reference, &models.OpenEscrowDisputeRequest{
		Reason:  "Item not delivered",
		Details: "The seller has not shipped the item",
	})
	if err != nil {
		t.Fatalf("open dispute: %v", err)
	}
	if disputed.Status != "DISPUTED" {
		t.Fatalf("got dispute status %q", disputed.Status)
	}
	if len(disputed.Disputes) != 1 {
		t.Fatalf("expected 1 dispute, got %d", len(disputed.Disputes))
	}

	refunded, err := escrowService.RefundOrder(seller.ID, order.Reference, &models.RefundEscrowOrderRequest{
		Reason: "Seller accepted refund",
	})
	if err != nil {
		t.Fatalf("refund order: %v", err)
	}
	if refunded.Status != "REFUNDED" {
		t.Fatalf("got refund status %q", refunded.Status)
	}

	buyerWallet, err := walletService.GetWalletByUserID(buyer.ID)
	if err != nil {
		t.Fatalf("get buyer wallet: %v", err)
	}
	sellerWallet, err := walletService.GetWalletByUserID(seller.ID)
	if err != nil {
		t.Fatalf("get seller wallet: %v", err)
	}
	escrowWallet, err := walletService.GetWalletByReference("SYSTEM", "ESCROW_NGN")
	if err != nil {
		t.Fatalf("get escrow wallet: %v", err)
	}

	if buyerWallet.Balance != money.FromMinorUnits(250000) {
		t.Fatalf("got buyer balance %d", buyerWallet.Balance)
	}
	if sellerWallet.Balance != money.Zero {
		t.Fatalf("got seller balance %d", sellerWallet.Balance)
	}
	if escrowWallet.Balance != money.Zero {
		t.Fatalf("got escrow balance %d", escrowWallet.Balance)
	}
}

func TestEscrowDisputeResolvedByAdminRefund(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)
	walletService := NewWalletService(db)
	limitService := NewTransactionLimitService(db)
	escrowService := NewEscrowService(db, userService, walletService, limitService)

	seller := createTestUserWithWallet(t, db, userService, walletService, "08000000013", "3333", money.Zero)
	buyer := createTestUserWithWallet(t, db, userService, walletService, "08000000014", "4444", money.FromMinorUnits(200000))

	order, err := escrowService.CreateOrder(seller.ID, &models.CreateEscrowOrderRequest{
		Title:    "Phone case",
		Amount:   money.FromMinorUnits(100000),
		Currency: "NGN",
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if _, err := escrowService.FundOrder(buyer.ID, order.Reference, &models.FundEscrowOrderRequest{Pin: "4444"}); err != nil {
		t.Fatalf("fund order: %v", err)
	}
	if _, err := escrowService.OpenDispute(buyer.ID, order.Reference, &models.OpenEscrowDisputeRequest{
		Reason: "Wrong item",
	}); err != nil {
		t.Fatalf("open dispute: %v", err)
	}

	resolved, err := escrowService.ResolveDispute(order.Reference, &models.ResolveEscrowDisputeRequest{
		Action:         "REFUND",
		ResolutionNote: "Admin sided with buyer",
	})
	if err != nil {
		t.Fatalf("resolve dispute: %v", err)
	}
	if resolved.Status != "REFUNDED" {
		t.Fatalf("got status %q", resolved.Status)
	}
	if len(resolved.Disputes) != 1 || resolved.Disputes[0].Status != "RESOLVED_REFUNDED" {
		t.Fatalf("unexpected dispute state %+v", resolved.Disputes)
	}
}

func TestWebhookFundsEscrowPayin(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)
	walletService := NewWalletService(db)
	limitService := NewTransactionLimitService(db)
	escrowService := NewEscrowService(db, userService, walletService, limitService)
	webhookService := NewWebhookService(db, escrowService)

	seller := createTestUserWithWallet(t, db, userService, walletService, "08000000015", "5555", money.Zero)
	buyer := createTestUserWithWallet(t, db, userService, walletService, "08000000016", "6666", money.Zero)

	order, err := escrowService.CreateOrder(seller.ID, &models.CreateEscrowOrderRequest{
		Title:    "Laptop bag",
		Amount:   money.FromMinorUnits(180000),
		Currency: "NGN",
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	providerTx := models.ProviderTransaction{
		Provider:      "kora",
		Reference:     "ESCROW-CHECKOUT-1",
		UserID:        &buyer.ID,
		EscrowOrderID: mustUUIDPtr(order.ID),
		Type:          "ESCROW_PAYIN",
		Status:        "PENDING",
		Amount:        money.FromMinorUnits(180000),
		Currency:      "NGN",
	}
	if err := db.Create(&providerTx).Error; err != nil {
		t.Fatalf("create provider tx: %v", err)
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"event": "charge.success",
		"data": map[string]interface{}{
			"reference": "ESCROW-CHECKOUT-1",
			"status":    "success",
		},
	})
	processed, err := webhookService.Record(WebhookInput{
		Provider:  "kora",
		EventID:   "evt-escrow-1",
		EventType: "charge.success",
		Reference: "ESCROW-CHECKOUT-1",
		Payload:   payload,
	})
	if err != nil {
		t.Fatalf("record webhook: %v", err)
	}
	if !processed {
		t.Fatalf("expected webhook to be processed")
	}

	orderAfter, err := escrowService.GetOrder(seller.ID, order.Reference)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if orderAfter.Status != "FUNDED" {
		t.Fatalf("got order status %q", orderAfter.Status)
	}
	if orderAfter.BuyerID != buyer.ID.String() {
		t.Fatalf("got buyer id %q", orderAfter.BuyerID)
	}

	escrowWallet, err := walletService.GetWalletByReference("SYSTEM", "ESCROW_NGN")
	if err != nil {
		t.Fatalf("get escrow wallet: %v", err)
	}
	if escrowWallet.Balance != money.FromMinorUnits(180000) {
		t.Fatalf("got escrow balance %d", escrowWallet.Balance)
	}
}

func TestWebhookFundsSellerGeneratedEscrowPayinWithoutBuyerUser(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)
	walletService := NewWalletService(db)
	limitService := NewTransactionLimitService(db)
	escrowService := NewEscrowService(db, userService, walletService, limitService)
	webhookService := NewWebhookService(db, escrowService)

	seller := createTestUserWithWallet(t, db, userService, walletService, "08000000017", "7777", money.Zero)

	order, err := escrowService.CreateOrder(seller.ID, &models.CreateEscrowOrderRequest{
		Title:    "Instagram bag",
		Amount:   money.FromMinorUnits(210000),
		Currency: "NGN",
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	requestPayload, _ := json.Marshal(map[string]interface{}{
		"escrow_reference":     order.Reference,
		"checkout_source":      "seller_share_link",
		"initiated_by_user_id": seller.ID.String(),
		"buyer_name":           "External Buyer",
		"buyer_email":          "external@example.com",
		"buyer_phone":          "08035550000",
	})
	providerTx := models.ProviderTransaction{
		Provider:       "kora",
		Reference:      "ESCROW-SELLER-LINK-1",
		EscrowOrderID:  mustUUIDPtr(order.ID),
		Type:           "ESCROW_PAYIN",
		Status:         "PENDING",
		Amount:         money.FromMinorUnits(210000),
		Currency:       "NGN",
		RequestPayload: json.RawMessage(requestPayload),
	}
	if err := db.Create(&providerTx).Error; err != nil {
		t.Fatalf("create provider tx: %v", err)
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"event": "charge.success",
		"data": map[string]interface{}{
			"reference": "ESCROW-SELLER-LINK-1",
			"status":    "success",
		},
	})
	processed, err := webhookService.Record(WebhookInput{
		Provider:  "kora",
		EventID:   "evt-escrow-seller-link-1",
		EventType: "charge.success",
		Reference: "ESCROW-SELLER-LINK-1",
		Payload:   payload,
	})
	if err != nil {
		t.Fatalf("record webhook: %v", err)
	}
	if !processed {
		t.Fatalf("expected webhook to be processed")
	}

	orderAfter, err := escrowService.GetOrder(seller.ID, order.Reference)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if orderAfter.Status != "FUNDED" {
		t.Fatalf("got order status %q", orderAfter.Status)
	}
	if orderAfter.BuyerID != "" {
		t.Fatalf("external checkout should not assign buyer id, got %q", orderAfter.BuyerID)
	}
	if orderAfter.BuyerEmail != "external@example.com" {
		t.Fatalf("buyer email = %q", orderAfter.BuyerEmail)
	}

	var orderRow models.EscrowOrder
	if err := db.Where("id = ?", order.ID).First(&orderRow).Error; err != nil {
		t.Fatalf("find order row: %v", err)
	}
	if orderRow.BuyerID != nil || orderRow.BuyerWalletID != nil {
		t.Fatalf("external checkout should leave buyer pointers nil: buyer=%v wallet=%v", orderRow.BuyerID, orderRow.BuyerWalletID)
	}

	escrowWallet, err := walletService.GetWalletByReference("SYSTEM", "ESCROW_NGN")
	if err != nil {
		t.Fatalf("get escrow wallet: %v", err)
	}
	if escrowWallet.Balance != money.FromMinorUnits(210000) {
		t.Fatalf("got escrow balance %d", escrowWallet.Balance)
	}

	_, err = escrowService.RefundOrder(seller.ID, order.Reference, &models.RefundEscrowOrderRequest{
		Reason: "Buyer requested refund",
	})
	if err == nil {
		t.Fatalf("expected external refund guard error")
	}
	if err.Error() != "escrow order was funded through external Kora checkout; initiate a Kora refund instead" {
		t.Fatalf("unexpected refund error: %v", err)
	}
}

func setupEscrowTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Wallet{},
		&models.Transaction{},
		&models.LedgerEntry{},
		&models.TransactionLimit{},
		&models.TransactionUsage{},
		&models.ProviderTransaction{},
		&models.WebhookEvent{},
		&models.EscrowOrder{},
		&models.EscrowEvent{},
		&models.EscrowDispute{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func createTestUserWithWallet(t *testing.T, db *gorm.DB, userService *UserService, walletService *WalletService, phone, pin string, balance money.Amount) *models.User {
	t.Helper()

	user, err := userService.CreateUser(&models.RegisterRequest{
		Phone:     phone,
		Password:  "password1",
		FirstName: "Test",
		LastName:  "User",
		Pin:       pin,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	wallet, err := walletService.CreateWallet(user.ID, "NGN")
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}
	if err := db.Model(wallet).Update("balance", balance).Error; err != nil {
		t.Fatalf("seed wallet balance: %v", err)
	}
	return user
}

func mustUUIDPtr(id uuid.UUID) *uuid.UUID {
	return &id
}
