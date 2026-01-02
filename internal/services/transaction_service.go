package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/utils"
)

type TransactionService struct {
	db            *gorm.DB
	userService   *UserService
	walletService *WalletService
}

func NewTransactionService(db *gorm.DB, userService *UserService, walletService *WalletService) *TransactionService {
	return &TransactionService{
		db:            db,
		userService:   userService,
		walletService: walletService,
	}
}

// Generate Reference creates a unique transaction reference
func (s *TransactionService) generateReference() string {
	return fmt.Sprintf("TRX%s", uuid.New().String()[:8]) // More reliable
}

// Transfer handeles P2P money transfer with doubele-entry bookeeping
func (s *TransactionService) Transfer(userID uuid.UUID, req *models.TransferRequest) (*models.TransferResponse, error) {

	const minAmount = 100
	if req.Amount <= minAmount {
		return nil, errors.New("Transfer amount is below the minimum limit")
	}

	const maxTransferAmount = 1000000
	if req.Amount > maxTransferAmount {
		return nil, errors.New("Transfer amount exceeds max limit")
	}
	//Start database transaction

	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}

	//Defer rollback in case of panic
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 1. Verify user's Pin
	user, err := s.userService.GetUserByID(userID.String())
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := utils.CheckPassword(user.PinHash, req.Pin); err != nil {
		tx.Rollback()
		return nil, errors.New("Invalid PIN")
	}

	// 2.Find recipient's wallet
	recipient, err := s.userService.GetUserByWallet(req.RecipientWalletID)
	if err != nil {
		tx.Rollback()
		return nil, errors.New("Recipient wallet not found")
	}

	// 3.Prevent self transfer
	if recipient.ID == userID {
		tx.Rollback()
		return nil, errors.New("Cannot transfer to self")
	}

	// 4. Get sender's wallet with pessimistic lock (FOR UPDATE)
	// var senderWallet models.Wallet
	// if err := tx.Clauses(
	// 	//This locks the row until transaction completes
	// 	clause.Locking{Strength: "UPDATE"},
	// ).Where("user_id = ?", userID).First(&senderWallet).Error; err != nil {
	// 	tx.Rollback()
	// 	if errors.Is(err, gorm.ErrRecordNotFound) {
	// 		return nil, errors.New("Sender wallet not found")
	// 	}
	// 	return nil, err
	// }
	// // 5 Get recipient's wallet with pessimistic lock (FOR UPDATE)
	// var recipientWallet models.Wallet
	// if err := tx.Clauses(
	// 	clause.Locking{Strength: "UPDATE"},
	// ).Where("user_id = ?", recipient.ID).First(&recipientWallet).Error; err != nil {
	// 	tx.Rollback()
	// 	if errors.Is(err, gorm.ErrRecordNotFound) {
	// 		return nil, errors.New("Recipient wallet not found")
	// 	}
	// 	return nil, err
	// }

	// 4 & 5. Get wallets with pessimistic lock in consistent order (prevent deadlock)
	var senderWallet, recipientWallet models.Wallet

	// Determine locking order based on user IDs (always lock in ascending order)
	lockSenderFirst := userID.String() < recipient.ID.String()

	if lockSenderFirst {
		// Lock sender wallet first
		if err := tx.Clauses(
			clause.Locking{Strength: "UPDATE"},
		).Where("user_id = ?", userID).First(&senderWallet).Error; err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("Sender wallet not found")
			}
			return nil, err
		}

		// Then lock recipient wallet
		if err := tx.Clauses(
			clause.Locking{Strength: "UPDATE"},
		).Where("user_id = ?", recipient.ID).First(&recipientWallet).Error; err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("Recipient wallet not found")
			}
			return nil, err
		}
	} else {
		// Lock recipient wallet first
		if err := tx.Clauses(
			clause.Locking{Strength: "UPDATE"},
		).Where("user_id = ?", recipient.ID).First(&recipientWallet).Error; err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("Recipient wallet not found")
			}
			return nil, err
		}

		// Then lock sender wallet
		if err := tx.Clauses(
			clause.Locking{Strength: "UPDATE"},
		).Where("user_id = ?", userID).First(&senderWallet).Error; err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("Sender wallet not found")
			}
			return nil, err
		}
	}

	// 6. Validate sufficient balance
	if senderWallet.Balance < req.Amount {
		tx.Rollback()
		return nil, errors.New("Insufficient funds")
	}

	//Create transaction record
	reference := s.generateReference()
	transaction := models.Transaction{
		Reference:    reference,
		FromWalletID: &senderWallet.ID,
		ToWalletID:   &recipientWallet.ID,
		Amount:       req.Amount,
		Currency:     "NGN",
		Type:         "TRANSFER",
		Status:       "PENDING",
		Description:  req.Description,
	}

	if req.Description == "" {
		transaction.Description = fmt.Sprintf("Transfer to %s", recipient.WalletID)
	}

	if err := tx.Create(&transaction).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// 8. Calculate new balances
	newSenderBalance := senderWallet.Balance - req.Amount
	newRecipientBalance := recipientWallet.Balance + req.Amount

	//9. Update wallet balances
	if err := tx.Model(&senderWallet).Update("balance", newSenderBalance).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Model(&recipientWallet).Update("balance", newRecipientBalance).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// 10. Create ledger entries (double-entry bookeeping)

	//Debit entry (sender)
	debitEntry := models.LedgerEntry{
		TransactionID: transaction.ID,
		WalletID:      senderWallet.ID,
		Debit:         req.Amount,
		Credit:        0,
		BalanceAfter:  newSenderBalance,
	}

	if err := tx.Create(&debitEntry).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	// Credit entry (recipient)
	creditEntry := models.LedgerEntry{
		TransactionID: transaction.ID,
		WalletID:      recipientWallet.ID,
		Debit:         0,
		Credit:        req.Amount,
		BalanceAfter:  newRecipientBalance,
	}

	if err := tx.Create(&creditEntry).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// 11 Update transaction status to COMPLETED
	completedAt := time.Now()
	if err := tx.Model(&transaction).Updates(map[string]interface{}{
		"status":       "COMPLETED",
		"completed_at": completedAt,
	}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	//12,. Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	// 13 Return response
	recipientName := fmt.Sprintf("%s %s", recipient.FirstName, recipient.LastName)
	if recipientName == " " {
		recipientName = ""
	}

	return &models.TransferResponse{
		Success: true,
		Transaction: models.TransactionDetail{
			ID:        transaction.ID.String(),
			Reference: transaction.Reference,
			Amount:    transaction.Amount,
			Recipient: models.Recipient{
				WalletID: recipient.WalletID.String(),
				Name:     recipientName,
			},
			Status:    transaction.Status,
			CreatedAt: transaction.CreatedAt.Format(time.RFC3339),
		},
		NewBalance: newSenderBalance,
	}, nil
}

func (s *TransactionService) GetTransactionHistory(userID uuid.UUID, limit int) ([]models.TransactionHistoryResponse, error) {
	// Get user's wallet
	wallet, err := s.walletService.GetWalletByUserID(userID)
	if err != nil {
		return nil, err
	}

	//Get transactions where user is sender or receiver
	var transactions []models.Transaction
	if err := s.db.Where("from_wallet_id = ? OR to_wallet_id = ?", wallet.ID, wallet.ID).
		Order("created_at DESC").
		Limit(limit).
		Find(&transactions).Error; err != nil {
		return nil, err
	}

	//Map to response
	history := make([]models.TransactionHistoryResponse, len(transactions))
	for i, tx := range transactions {
		txType := "CREDIT"
		if tx.FromWalletID != nil && *tx.FromWalletID == wallet.ID {
			txType = "DEBIT"
		}

		history[i] = models.TransactionHistoryResponse{
			ID:          tx.ID.String(),
			Reference:   tx.Reference,
			Amount:      tx.Amount,
			Type:        txType,
			Status:      tx.Status,
			Description: tx.Description,
			CreatedAt:   tx.CreatedAt,
		}
	}

	return history, nil
}

//Get TransactionByReference retrieves a transaction by reference

func (s *TransactionService) GetTransactionByReference(reference string) (*models.Transaction, error) {
	var transaction models.Transaction
	if err := s.db.Where("reference = ?", reference).
		Preload("FromWallet").
		Preload("ToWallet").
		First(&transaction).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("transaction not found")
		}
		return nil, err
	}
	return &transaction, nil
}
