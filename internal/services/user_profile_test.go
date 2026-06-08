//go:build cgo
// +build cgo

package services

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/pkg/money"
)

func TestUserProfileIncludesMerchantDetailsAndMetrics(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)
	walletService := NewWalletService(db)

	seller, err := userService.CreateUser(&models.RegisterRequest{
		Phone:     "08000000921",
		Password:  "password1",
		FirstName: "Test",
		LastName:  "Seller",
		Pin:       "1234",
		Role:      "seller",
	})
	if err != nil {
		t.Fatalf("create seller: %v", err)
	}
	sellerWallet, err := walletService.CreateWallet(seller.ID, "NGN")
	if err != nil {
		t.Fatalf("create seller wallet: %v", err)
	}
	buyer := createTestUserWithWallet(t, db, userService, walletService, "08000000922", "1234", money.Zero)

	if err := db.Model(seller).Updates(map[string]interface{}{
		"rating":      4.7,
		"trust_score": 73,
	}).Error; err != nil {
		t.Fatalf("seed seller rating: %v", err)
	}
	if err := userService.UpdateMerchantDetails(seller.ID.String(), &models.UpdateMerchantDetailsRequest{
		BusinessName: "Horology House Ltd.",
		BusinessType: "Private Limited Company",
		RCNumber:     "RC 1782940",
		Website:      "www.horologyhouse.com",
		Address:      "12 Marina, Lagos Island",
		City:         "Lagos",
		Country:      "Nigeria",
		SupportPhone: "08012345678",
	}); err != nil {
		t.Fatalf("update merchant details: %v", err)
	}

	now := time.Now().UTC()
	releasedOrder := models.EscrowOrder{
		ID:             uuid.New(),
		Reference:      "ESC-PROFILE-1",
		SellerID:       seller.ID,
		BuyerID:        &buyer.ID,
		SellerWalletID: sellerWallet.ID,
		Amount:         money.FromMinorUnits(100000),
		Currency:       "NGN",
		Title:          "Released order",
		DeliveryMode:   "PHYSICAL",
		Status:         "RELEASED",
		ReleasedAt:     &now,
	}
	disputedOrder := models.EscrowOrder{
		ID:             uuid.New(),
		Reference:      "ESC-PROFILE-2",
		SellerID:       seller.ID,
		BuyerID:        &buyer.ID,
		SellerWalletID: sellerWallet.ID,
		Amount:         money.FromMinorUnits(200000),
		Currency:       "NGN",
		Title:          "Disputed order",
		DeliveryMode:   "PHYSICAL",
		Status:         "DISPUTED",
		DisputedAt:     &now,
	}
	if err := db.Create(&releasedOrder).Error; err != nil {
		t.Fatalf("create released order: %v", err)
	}
	if err := db.Create(&disputedOrder).Error; err != nil {
		t.Fatalf("create disputed order: %v", err)
	}
	if err := db.Create(&models.EscrowDispute{
		ID:             uuid.New(),
		EscrowID:       disputedOrder.ID,
		RaisedByUserID: buyer.ID,
		Reason:         "Wrong item",
		Status:         "OPEN",
	}).Error; err != nil {
		t.Fatalf("create dispute: %v", err)
	}

	profile, err := userService.GetProfile(seller.ID.String())
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}

	if profile.Role != "seller" {
		t.Fatalf("got role %q", profile.Role)
	}
	if profile.MerchantDetails.BusinessName != "Horology House Ltd." {
		t.Fatalf("got business name %q", profile.MerchantDetails.BusinessName)
	}
	if profile.MerchantDetails.SupportPhone != "08012345678" {
		t.Fatalf("got support phone %q", profile.MerchantDetails.SupportPhone)
	}
	if profile.Metrics.CompletedOrders != 1 {
		t.Fatalf("completed orders = %d", profile.Metrics.CompletedOrders)
	}
	if profile.Metrics.SalesVolume != money.FromMinorUnits(100000) {
		t.Fatalf("sales volume = %d", profile.Metrics.SalesVolume)
	}
	if profile.Metrics.TrustScore != 50 {
		t.Fatalf("trust score = %d", profile.Metrics.TrustScore)
	}
	if profile.Metrics.Rating != 4.7 {
		t.Fatalf("rating = %v", profile.Metrics.Rating)
	}
}

func TestBuyerProfileMetricsUseBuyerOrders(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)
	walletService := NewWalletService(db)

	seller := createTestUserWithWallet(t, db, userService, walletService, "08000000923", "1234", money.Zero)
	sellerWallet, err := walletService.GetWalletByUserID(seller.ID)
	if err != nil {
		t.Fatalf("get seller wallet: %v", err)
	}
	buyer := createTestUserWithWallet(t, db, userService, walletService, "08000000924", "1234", money.Zero)

	now := time.Now().UTC()
	order := models.EscrowOrder{
		ID:             uuid.New(),
		Reference:      "ESC-BUYER-PROFILE-1",
		SellerID:       seller.ID,
		BuyerID:        &buyer.ID,
		SellerWalletID: sellerWallet.ID,
		Amount:         money.FromMinorUnits(70000),
		Currency:       "NGN",
		Title:          "Buyer order",
		DeliveryMode:   "PHYSICAL",
		Status:         "RELEASED",
		ReleasedAt:     &now,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}

	profile, err := userService.GetProfile(buyer.ID.String())
	if err != nil {
		t.Fatalf("get buyer profile: %v", err)
	}
	if profile.Role != "buyer" {
		t.Fatalf("got role %q", profile.Role)
	}
	if profile.Metrics.CompletedOrders != 1 {
		t.Fatalf("completed orders = %d", profile.Metrics.CompletedOrders)
	}
	if profile.Metrics.SalesVolume != money.Zero {
		t.Fatalf("buyer sales volume = %d", profile.Metrics.SalesVolume)
	}
	if profile.Metrics.TrustScore != 100 {
		t.Fatalf("buyer trust score = %d", profile.Metrics.TrustScore)
	}
}
