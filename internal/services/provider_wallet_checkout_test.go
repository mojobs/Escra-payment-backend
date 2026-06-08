//go:build cgo
// +build cgo

package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/providers/kora"
	"github.com/mojobs/lara-payment-backend.git/pkg/money"
)

type fakeWalletCheckoutKoraClient struct {
	checkoutRequest kora.CheckoutRequest
}

func (f *fakeWalletCheckoutKoraClient) ListBanks(context.Context, string) ([]kora.Bank, error) {
	return nil, nil
}

func (f *fakeWalletCheckoutKoraClient) ResolveBankAccount(context.Context, kora.ResolveBankAccountRequest) (*kora.ResolvedBankAccount, error) {
	return nil, nil
}

func (f *fakeWalletCheckoutKoraClient) VerifyIdentity(context.Context, kora.VerifyIdentityRequest) (*kora.VerifyIdentityResponse, error) {
	return nil, nil
}

func (f *fakeWalletCheckoutKoraClient) CreateVirtualAccount(context.Context, kora.VirtualAccountRequest) (*kora.VirtualAccountResponse, error) {
	return nil, nil
}

func (f *fakeWalletCheckoutKoraClient) GetBalances(context.Context) ([]kora.Balance, error) {
	return nil, nil
}

func (f *fakeWalletCheckoutKoraClient) RequestPayout(context.Context, kora.PayoutRequest) (*kora.PayoutResponse, error) {
	return nil, nil
}

func (f *fakeWalletCheckoutKoraClient) RequestRefund(context.Context, kora.RefundRequest) (*kora.RefundResponse, error) {
	return nil, nil
}

func (f *fakeWalletCheckoutKoraClient) InitializeCheckout(_ context.Context, req kora.CheckoutRequest) (*kora.CheckoutResponse, error) {
	f.checkoutRequest = req
	return &kora.CheckoutResponse{
		Reference:   req.Reference,
		CheckoutURL: "https://checkout.korapay.com/pay/test",
		Status:      "pending",
		Raw:         json.RawMessage(`{"status":true,"data":{"checkout_url":"https://checkout.korapay.com/pay/test","status":"pending"}}`),
	}, nil
}

func TestInitiateWalletKoraCheckout(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)
	walletService := NewWalletService(db)
	limitService := NewTransactionLimitService(db)
	fakeKora := &fakeWalletCheckoutKoraClient{}
	providerService := NewProviderService(db, userService, walletService, limitService, fakeKora, nil)

	user := createTestUserWithWallet(t, db, userService, walletService, "08000000901", "1234", money.Zero)
	response, err := providerService.InitiateWalletKoraCheckout(context.Background(), user.ID, &models.KoraWalletCheckoutRequest{
		Amount:      money.FromMinorUnits(500000),
		RedirectURL: "https://escra.app/wallet/callback",
		Narration:   "Direct wallet deposit",
	}, "https://api.escra.test")
	if err != nil {
		t.Fatalf("initiate wallet checkout: %v", err)
	}

	if !response.Success {
		t.Fatalf("expected success")
	}
	if response.CheckoutURL == "" {
		t.Fatalf("expected checkout url")
	}
	if response.Reference == "" {
		t.Fatalf("expected reference")
	}
	if fakeKora.checkoutRequest.Reference != response.Reference {
		t.Fatalf("checkout request reference %q response %q", fakeKora.checkoutRequest.Reference, response.Reference)
	}
	if fakeKora.checkoutRequest.NotificationURL != "https://api.escra.test/api/v1/webhooks/kora" {
		t.Fatalf("unexpected notification url %q", fakeKora.checkoutRequest.NotificationURL)
	}
	if fakeKora.checkoutRequest.Narration != "Direct wallet deposit" {
		t.Fatalf("unexpected narration %q", fakeKora.checkoutRequest.Narration)
	}
	if fakeKora.checkoutRequest.CustomerEmail == "" {
		t.Fatalf("expected provider customer email fallback")
	}

	var providerTx models.ProviderTransaction
	if err := db.Where("reference = ?", response.Reference).First(&providerTx).Error; err != nil {
		t.Fatalf("find provider tx: %v", err)
	}
	if providerTx.Type != "WALLET_DEPOSIT" || providerTx.Status != "PROCESSING" {
		t.Fatalf("unexpected provider tx type/status %s/%s", providerTx.Type, providerTx.Status)
	}
	if providerTx.TransactionID == nil {
		t.Fatalf("expected linked internal transaction")
	}
}

func TestWebhookCreditsWalletDepositOnce(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)
	walletService := NewWalletService(db)
	webhookService := NewWebhookService(db, nil)

	user := createTestUserWithWallet(t, db, userService, walletService, "08000000902", "1234", money.Zero)
	wallet, err := walletService.GetWalletByUserID(user.ID)
	if err != nil {
		t.Fatalf("get wallet: %v", err)
	}

	transaction := models.Transaction{
		ID:          uuid.New(),
		Reference:   "DEP-KORA-1",
		ToWalletID:  &wallet.ID,
		Amount:      money.FromMinorUnits(500000),
		Currency:    "NGN",
		Type:        "TOP_UP",
		Status:      "PENDING",
		Description: "Direct wallet deposit",
	}
	if err := db.Create(&transaction).Error; err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	providerTx := models.ProviderTransaction{
		ID:            uuid.New(),
		Provider:      "kora",
		Reference:     "DEP-KORA-1",
		UserID:        &user.ID,
		TransactionID: &transaction.ID,
		Type:          "WALLET_DEPOSIT",
		Status:        "PENDING",
		Amount:        money.FromMinorUnits(500000),
		Currency:      "NGN",
	}
	if err := db.Create(&providerTx).Error; err != nil {
		t.Fatalf("create provider tx: %v", err)
	}

	payload := json.RawMessage(`{"event":"charge.success","data":{"reference":"DEP-KORA-1","status":"success","amount":"5000.00"}}`)
	if processed, err := webhookService.Record(WebhookInput{
		Provider:  "kora",
		EventID:   "evt-wallet-1",
		EventType: "charge.success",
		Reference: "DEP-KORA-1",
		Payload:   payload,
	}); err != nil {
		t.Fatalf("record webhook: %v", err)
	} else if !processed {
		t.Fatalf("expected first webhook to process")
	}

	wallet, err = walletService.GetWalletByUserID(user.ID)
	if err != nil {
		t.Fatalf("get credited wallet: %v", err)
	}
	if wallet.Balance != money.FromMinorUnits(500000) {
		t.Fatalf("wallet balance = %d", wallet.Balance)
	}

	var completed models.Transaction
	if err := db.Where("id = ?", transaction.ID).First(&completed).Error; err != nil {
		t.Fatalf("find transaction: %v", err)
	}
	if completed.Status != "COMPLETED" || completed.CompletedAt == nil {
		t.Fatalf("transaction not completed: %+v", completed)
	}

	if processed, err := webhookService.Record(WebhookInput{
		Provider:  "kora",
		EventID:   "evt-wallet-2",
		EventType: "charge.success",
		Reference: "DEP-KORA-1",
		Payload:   payload,
	}); err != nil {
		t.Fatalf("record duplicate webhook: %v", err)
	} else if !processed {
		t.Fatalf("expected second webhook event to be recorded")
	}

	wallet, err = walletService.GetWalletByUserID(user.ID)
	if err != nil {
		t.Fatalf("get wallet after duplicate: %v", err)
	}
	if wallet.Balance != money.FromMinorUnits(500000) {
		t.Fatalf("duplicate webhook changed wallet balance to %d", wallet.Balance)
	}

	var ledgerCount int64
	if err := db.Model(&models.LedgerEntry{}).Where("transaction_id = ?", transaction.ID).Count(&ledgerCount).Error; err != nil {
		t.Fatalf("count ledger entries: %v", err)
	}
	if ledgerCount != 1 {
		t.Fatalf("ledger entries = %d", ledgerCount)
	}
}
