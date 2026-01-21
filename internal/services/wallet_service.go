package services

import (
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"github.com/mojobs/lara-payment-backend.git/internal/models"
)

type WalletService struct {
	db *gorm.DB
}

func NewWalletService(db *gorm.DB) *WalletService{
	return &WalletService{db: db}

}

func (s *WalletService) CreateWallet(userID uuid.UUID, currency string) (*models.Wallet, error){
	wallet := &models.Wallet{
		UserID :userID,
		Balance : 0,
		Currency : currency,
		Status : "ACTIVE",
	}

	if err := s.db.Create(wallet).Error; err != nil {
		return nil, err
	}

	return wallet, nil
}

func (s *WalletService) GetWalletByUserID(userID uuid.UUID) (*models.Wallet, error) {
	var wallet models.Wallet
	if err := s.db.Where("user_id = ?", userID).First(&wallet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound){
			return nil, errors.New("Wallet not found")
		}
		return nil, err
	}
	return &wallet, nil
}

func (s *WalletService) GetWalletByID(id uuid.UUID) (*models.Wallet, error) {
	var wallet models.Wallet
	if err := s.db.Where("id = ?", id).First(&wallet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound){
			return nil, errors.New("Wallet not found")
		}
		return nil, err
	}
	return &wallet, nil
}

func (s *WalletService) GetBalance(userID uuid.UUID) (*models.BalanceResponse, error){
	wallet, err := s.GetWalletByUserID(userID)
	if err != nil {
		return nil, err
	}

	return &models.BalanceResponse{
		Balance : wallet.Balance,
		Currency : wallet.Currency,
	}, nil
}
