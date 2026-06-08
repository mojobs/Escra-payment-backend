package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/pkg/money"
)

type WebhookService struct {
	db            *gorm.DB
	escrowService *EscrowService
}

func NewWebhookService(db *gorm.DB, escrowService *EscrowService) *WebhookService {
	return &WebhookService{
		db:            db,
		escrowService: escrowService,
	}
}

type WebhookInput struct {
	Provider  string
	EventID   string
	EventType string
	Reference string
	Payload   json.RawMessage
}

func (s *WebhookService) Record(input WebhookInput) (bool, error) {
	if input.Provider == "" {
		return false, errors.New("provider is required")
	}
	if len(input.Payload) == 0 {
		return false, errors.New("webhook payload is required")
	}
	if input.EventID == "" {
		input.EventID = stableWebhookID(input.Provider, input.EventType, input.Reference, input.Payload)
	}
	if input.EventType == "" {
		input.EventType = "unknown"
	}

	var existing models.WebhookEvent
	err := s.db.Where("provider = ? AND event_id = ?", input.Provider, input.EventID).First(&existing).Error
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}

	event := models.WebhookEvent{
		Provider:  input.Provider,
		EventID:   input.EventID,
		EventType: input.EventType,
		Reference: input.Reference,
		Status:    "RECEIVED",
		Payload:   input.Payload,
	}

	if err := s.db.Create(&event).Error; err != nil {
		return false, err
	}

	if err := s.applyProviderWebhook(input); err != nil {
		return false, err
	}

	return true, nil
}

func (s *WebhookService) applyProviderWebhook(input WebhookInput) error {
	if input.Reference == "" && input.EventID == "" {
		return nil
	}

	status := providerStatusFromWebhook(input)
	if status == "" {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var providerTx models.ProviderTransaction
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("provider = ? AND (reference = ? OR external_reference = ?)", input.Provider, input.Reference, input.EventID).
			First(&providerTx).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}

		updates := map[string]interface{}{
			"status":           status,
			"response_payload": input.Payload,
		}
		if input.EventID != "" && providerTx.ExternalReference == "" {
			updates["external_reference"] = input.EventID
			providerTx.ExternalReference = input.EventID
		}

		if amountText := stringFromWebhook(input.Payload, "amount"); amountText != "" {
			if providerTx.Provider == "kora" {
				if parsed, err := money.ParseDecimal(amountText); err == nil {
					updates["amount"] = parsed
					providerTx.Amount = parsed
				}
			} else {
				updates["amount_text"] = amountText
				providerTx.AmountText = amountText
			}
		}
		if feeText := stringFromWebhook(input.Payload, "fee"); feeText != "" && providerTx.Provider == "kora" {
			if parsed, err := money.ParseDecimal(feeText); err == nil {
				updates["fee"] = parsed
				providerTx.Fee = parsed
			}
		}

		if err := tx.Model(&providerTx).Updates(updates).Error; err != nil {
			return err
		}

		if (providerTx.Type == "ESCROW_PAYIN" || providerTx.Type == "ESCROW_VIRTUAL_ACCOUNT") && providerTx.EscrowOrderID != nil && status == "SUCCESS" && s.escrowService != nil {
			if err := s.escrowService.FundOrderFromProviderTx(tx, *providerTx.EscrowOrderID, &providerTx); err != nil {
				return err
			}
		}
		if providerTx.Type == "WALLET_DEPOSIT" && status == "SUCCESS" {
			if err := creditWalletDepositFromProviderTx(tx, providerTx); err != nil {
				return err
			}
		}
		if providerTx.Type == "ESCROW_VIRTUAL_ACCOUNT" {
			if err := tx.Model(&models.KoraVirtualAccount{}).
				Where("provider_tx_id = ? OR account_reference = ?", providerTx.ID, providerTx.Reference).
				Updates(map[string]interface{}{
					"status":           status,
					"response_payload": input.Payload,
				}).Error; err != nil {
				return err
			}
		}

		if providerTx.TransactionID == nil {
			return nil
		}
		return updateInternalTransactionFromProvider(tx, providerTx, status)
	})
}

func creditWalletDepositFromProviderTx(tx *gorm.DB, providerTx models.ProviderTransaction) error {
	if providerTx.TransactionID == nil {
		return errors.New("wallet deposit provider transaction is missing internal transaction")
	}

	var transaction models.Transaction
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", *providerTx.TransactionID).
		First(&transaction).Error; err != nil {
		return err
	}
	if transaction.Status == "COMPLETED" {
		return nil
	}
	if transaction.ToWalletID == nil {
		return errors.New("wallet deposit transaction is missing destination wallet")
	}
	if providerTx.Amount.IsPositive() && providerTx.Amount != transaction.Amount {
		return fmt.Errorf("wallet deposit amount mismatch: provider=%s expected=%s", providerTx.Amount.String(), transaction.Amount.String())
	}

	var wallet models.Wallet
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", *transaction.ToWalletID).
		First(&wallet).Error; err != nil {
		return err
	}
	if wallet.Status != "ACTIVE" {
		return errors.New("wallet is not active")
	}

	newBalance := wallet.Balance + transaction.Amount
	if err := tx.Model(&wallet).Update("balance", newBalance).Error; err != nil {
		return err
	}

	ledgerEntry := models.LedgerEntry{
		TransactionID: transaction.ID,
		WalletID:      wallet.ID,
		Debit:         money.Zero,
		Credit:        transaction.Amount,
		BalanceAfter:  newBalance,
	}
	if err := tx.Create(&ledgerEntry).Error; err != nil {
		return err
	}

	completedAt := time.Now()
	return tx.Model(&transaction).Updates(map[string]interface{}{
		"status":       "COMPLETED",
		"completed_at": completedAt,
	}).Error
}

func updateInternalTransactionFromProvider(tx *gorm.DB, providerTx models.ProviderTransaction, status string) error {
	var transaction models.Transaction
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", *providerTx.TransactionID).
		First(&transaction).Error; err != nil {
		return err
	}

	switch status {
	case "SUCCESS":
		if transaction.Status == "COMPLETED" {
			return nil
		}
		completedAt := time.Now()
		return tx.Model(&transaction).Updates(map[string]interface{}{
			"status":       "COMPLETED",
			"completed_at": completedAt,
		}).Error
	case "FAILED":
		if transaction.Status == "FAILED" {
			return nil
		}
		if transaction.Type == "WITHDRAWAL" && transaction.FromWalletID != nil && providerTx.Amount.IsPositive() {
			var wallet models.Wallet
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ?", *transaction.FromWalletID).
				First(&wallet).Error; err != nil {
				return err
			}
			newBalance := wallet.Balance + providerTx.Amount
			if err := tx.Model(&wallet).Update("balance", newBalance).Error; err != nil {
				return err
			}
			refundEntry := models.LedgerEntry{
				TransactionID: transaction.ID,
				WalletID:      wallet.ID,
				Debit:         money.Zero,
				Credit:        providerTx.Amount,
				BalanceAfter:  newBalance,
			}
			if err := tx.Create(&refundEntry).Error; err != nil {
				return err
			}
		}
		return tx.Model(&transaction).Update("status", "FAILED").Error
	default:
		return tx.Model(&transaction).Update("status", "PENDING").Error
	}
}

func providerStatusFromWebhook(input WebhookInput) string {
	eventType := strings.ToLower(input.EventType)
	switch {
	case strings.Contains(eventType, "successful"), strings.Contains(eventType, "success"), strings.Contains(eventType, "completed"):
		return "SUCCESS"
	case strings.Contains(eventType, "rejected"), strings.Contains(eventType, "failed"), strings.Contains(eventType, "failure"):
		return "FAILED"
	}

	return normalizeProviderStatus(stringFromWebhook(input.Payload, "status"))
}

func stringFromWebhook(payload []byte, key string) string {
	var body map[string]interface{}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&body); err != nil {
		return ""
	}
	if value := scalarString(body[key]); value != "" {
		return value
	}
	if data, ok := body["data"].(map[string]interface{}); ok {
		if value := scalarString(data[key]); value != "" {
			return value
		}
	}
	return ""
}

func scalarString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func stableWebhookID(provider, eventType, reference string, payload []byte) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s:%x", provider, eventType, reference, sha256.Sum256(payload))))
	return hex.EncodeToString(sum[:])
}
