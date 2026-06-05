package services

import (
	"errors"

	"github.com/google/uuid"
	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/pkg/money"
	"gorm.io/gorm"
)

type WalletService struct {
	db *gorm.DB
}

func NewWalletService(db *gorm.DB) *WalletService {
	return &WalletService{db: db}

}

func (s *WalletService) CreateWallet(userID uuid.UUID, currency string) (*models.Wallet, error) {
	return s.createWallet(s.db, userID, currency)
}

func (s *WalletService) createWallet(tx *gorm.DB, userID uuid.UUID, currency string) (*models.Wallet, error) {
	ownerID := userID
	wallet := &models.Wallet{
		UserID:         &ownerID,
		OwnerType:      "USER",
		OwnerReference: userID.String(),
		Balance:        money.Zero,
		Currency:       currency,
		Status:         "ACTIVE",
	}

	if err := tx.Create(wallet).Error; err != nil {
		return nil, err
	}

	return wallet, nil
}

func (s *WalletService) GetOrCreateSystemWallet(reference, currency string) (*models.Wallet, error) {
	return s.getOrCreateSystemWallet(s.db, reference, currency)
}

func (s *WalletService) getOrCreateSystemWallet(tx *gorm.DB, reference, currency string) (*models.Wallet, error) {
	var wallet models.Wallet
	err := tx.Where("owner_type = ? AND owner_reference = ?", "SYSTEM", reference).First(&wallet).Error
	if err == nil {
		return &wallet, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	wallet = models.Wallet{
		OwnerType:      "SYSTEM",
		OwnerReference: reference,
		Balance:        money.Zero,
		Currency:       currency,
		Status:         "ACTIVE",
	}
	if err := tx.Create(&wallet).Error; err != nil {
		return nil, err
	}
	return &wallet, nil
}

func (s *WalletService) GetWalletByUserID(userID uuid.UUID) (*models.Wallet, error) {
	var wallet models.Wallet
	if err := s.db.Where("owner_type = ? AND user_id = ?", "USER", userID).First(&wallet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("Wallet not found")
		}
		return nil, err
	}
	return &wallet, nil
}

func (s *WalletService) GetWalletByID(id uuid.UUID) (*models.Wallet, error) {
	var wallet models.Wallet
	if err := s.db.Where("id = ?", id).First(&wallet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("Wallet not found")
		}
		return nil, err
	}
	return &wallet, nil
}

func (s *WalletService) GetWalletByReference(ownerType, reference string) (*models.Wallet, error) {
	var wallet models.Wallet
	if err := s.db.Where("owner_type = ? AND owner_reference = ?", ownerType, reference).First(&wallet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("Wallet not found")
		}
		return nil, err
	}
	return &wallet, nil
}

func (s *WalletService) GetBalance(userID uuid.UUID) (*models.BalanceResponse, error) {
	wallet, err := s.GetWalletByUserID(userID)
	if err != nil {
		return nil, err
	}

	return &models.BalanceResponse{
		BalanceKobo:      wallet.Balance,
		BalanceFormatted: wallet.Balance.String(),
		Currency:         wallet.Currency,
	}, nil
}
