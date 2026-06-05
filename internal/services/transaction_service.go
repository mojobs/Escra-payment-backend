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
	db                      *gorm.DB
	userService             *UserService
	walletService           *WalletService
	transactionLimitService *TransactionLimitService
}

func NewTransactionService(db *gorm.DB, userService *UserService, walletService *WalletService, limitService *TransactionLimitService) *TransactionService {
	return &TransactionService{
		db:                      db,
		userService:             userService,
		walletService:           walletService,
		transactionLimitService: limitService,
	}
}

// Generate Reference creates a unique transaction reference
func (s *TransactionService) generateReference() string {
	return fmt.Sprintf("TRX%s", uuid.New().String()[:8]) // More reliable
}

// Transfer handles P2P money transfer with double-entry bookkeeping.
func (s *TransactionService) Transfer(userID uuid.UUID, req *models.TransferRequest) (*models.TransferResponse, error) {
	if !req.Amount.IsPositive() {
		return nil, errors.New("amount must be greater than zero")
	}

	var transaction models.Transaction
	var recipient models.User
	var recipientWallet models.Wallet
	var newSenderBalance = req.Amount

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("user not found")
			}
			return err
		}

		if err := utils.CheckPassword(user.PinHash, req.Pin); err != nil {
			return errors.New("invalid PIN")
		}

		recipientWalletID, err := uuid.Parse(req.RecipientWalletID)
		if err != nil {
			return errors.New("invalid recipient wallet")
		}

		if err := tx.Where("id = ?", recipientWalletID).First(&recipientWallet).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("recipient wallet not found")
			}
			return err
		}

		if recipientWallet.UserID == nil {
			return errors.New("recipient wallet is not user owned")
		}

		if *recipientWallet.UserID == userID {
			return errors.New("cannot transfer to self")
		}

		if err := tx.Where("id = ?", *recipientWallet.UserID).First(&recipient).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("recipient not found")
			}
			return err
		}

		var senderWallet models.Wallet
		if err := s.lockWallets(tx, userID, recipient.ID, &senderWallet, &recipientWallet); err != nil {
			return err
		}

		if senderWallet.Status != "ACTIVE" {
			return errors.New("sender wallet is not active")
		}
		if recipientWallet.Status != "ACTIVE" {
			return errors.New("recipient wallet is not active")
		}

		if senderWallet.Balance < req.Amount {
			return errors.New("insufficient funds")
		}

		if err := s.transactionLimitService.CheckAndRecordTransaction(tx, userID, req.Amount); err != nil {
			return err
		}

		transaction = models.Transaction{
			Reference:    s.generateReference(),
			FromWalletID: &senderWallet.ID,
			ToWalletID:   &recipientWallet.ID,
			Amount:       req.Amount,
			Currency:     "NGN",
			Type:         "TRANSFER",
			Status:       "PENDING",
			Description:  req.Description,
		}

		if req.Description == "" {
			transaction.Description = fmt.Sprintf("Transfer to %s", recipientWallet.ID.String())
		}

		if err := tx.Create(&transaction).Error; err != nil {
			return err
		}

		newSenderBalance = senderWallet.Balance - req.Amount
		newRecipientBalance := recipientWallet.Balance + req.Amount

		if err := tx.Model(&senderWallet).Update("balance", newSenderBalance).Error; err != nil {
			return err
		}
		if err := tx.Model(&recipientWallet).Update("balance", newRecipientBalance).Error; err != nil {
			return err
		}

		ledgerEntries := []models.LedgerEntry{
			{
				TransactionID: transaction.ID,
				WalletID:      senderWallet.ID,
				Debit:         req.Amount,
				Credit:        0,
				BalanceAfter:  newSenderBalance,
			},
			{
				TransactionID: transaction.ID,
				WalletID:      recipientWallet.ID,
				Debit:         0,
				Credit:        req.Amount,
				BalanceAfter:  newRecipientBalance,
			},
		}
		if err := tx.Create(&ledgerEntries).Error; err != nil {
			return err
		}

		completedAt := time.Now()
		if err := tx.Model(&transaction).Updates(map[string]interface{}{
			"status":       "COMPLETED",
			"completed_at": completedAt,
		}).Error; err != nil {
			return err
		}
		transaction.Status = "COMPLETED"
		transaction.CompletedAt = &completedAt
		return nil
	})
	if err != nil {
		return nil, err
	}

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
				WalletID: recipientWallet.ID.String(),
				Name:     recipientName,
			},
			Status:    transaction.Status,
			CreatedAt: transaction.CreatedAt.Format(time.RFC3339),
		},
		NewBalance: newSenderBalance,
	}, nil
}

func (s *TransactionService) lockWallets(tx *gorm.DB, senderUserID, recipientUserID uuid.UUID, senderWallet, recipientWallet *models.Wallet) error {
	lock := clause.Locking{Strength: "UPDATE"}
	lockSenderFirst := senderUserID.String() < recipientUserID.String()

	if lockSenderFirst {
		if err := tx.Clauses(lock).Where("owner_type = ? AND user_id = ?", "USER", senderUserID).First(senderWallet).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("sender wallet not found")
			}
			return err
		}
		if err := tx.Clauses(lock).Where("owner_type = ? AND user_id = ?", "USER", recipientUserID).First(recipientWallet).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("recipient wallet not found")
			}
			return err
		}
		return nil
	}

	if err := tx.Clauses(lock).Where("owner_type = ? AND user_id = ?", "USER", recipientUserID).First(recipientWallet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("recipient wallet not found")
		}
		return err
	}
	if err := tx.Clauses(lock).Where("owner_type = ? AND user_id = ?", "USER", senderUserID).First(senderWallet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("sender wallet not found")
		}
		return err
	}
	return nil
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

// Get TransactionByReference retrieves a transaction by reference for the authenticated user.
func (s *TransactionService) GetTransactionByReference(userID uuid.UUID, reference string) (*models.Transaction, error) {
	wallet, err := s.walletService.GetWalletByUserID(userID)
	if err != nil {
		return nil, err
	}

	var transaction models.Transaction
	if err := s.db.Where("reference = ? AND (from_wallet_id = ? OR to_wallet_id = ?)", reference, wallet.ID, wallet.ID).
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
