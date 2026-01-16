package services

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
)

type TransactionLimitService struct {
	db *gorm.DB
}

func NewTransactionLimitService(db *gorm.DB) *TransactionLimitService {
	return &TransactionLimitService{db: db}
}

// CreateDefault Limit creates default limits for a new user

func (s *TransactionLimitService) CreateDefaultLimit(userID uuid.UUID) error {
	limit := models.TransactionLimit{
		UserID:               userID,
		MaxTransactionAmount: 100000,
		DailyLimit:           500000,
		MonthlyLimit:         2000000,
	}

	return s.db.Create(&limit).Error
}

// GetUserLimit retrieves user's Transaction Limits
func (s *TransactionLimitService) GetUserLimIT(userID uuid.UUID) (*models.TransactionLimit, error) {
	var limit models.TransactionLimit
	if err := s.db.Where("user_id = ?", userID).First(&limit).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			//create a deafult if doesn't exist
			if err := s.CreateDefaultLimit(userID); err != nil {
				return nil, err
			}
			return s.GetUserLimIT(userID)
		}
		return nil, err
	}
	return &limit, nil
}

func (s *TransactionLimitService) CheckTransactionLimit(userID uuid.UUID, amount float64) error {
	limit, err := s.GetUserLimIT(userID)
	if err != nil {
		return err
	}

	//Check per transaction
	if amount > limit.MaxTransactionAmount {
		return errors.New("amount exceeds maximum limit")
	}

	//Check daily limit
	today := time.Now().Truncate(24 * time.Hour)
	dailyUsage, err := s.getDailyUsage(userID, today)
	if err != nil {
		return err
	}
	if dailyUsage+amount > limit.DailyLimit {
		return errors.New("transaction would exceed daily limit")
	}

	//Check monthly limit
	monthStart := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.UTC)
	monthlyUsage, err := s.getMonthlyUsage(userID, monthStart)
	if err != nil {
		return err
	}
	if monthlyUsage+amount > limit.MonthlyLimit {
		return errors.New("transaction would exceed monthly limit")
	}
	return nil
}

// RecordTransaction records a successful transaction for limit tracking
func (s *TransactionLimitService) RecordTransaction(userID uuid.UUID, amount float64) error {
	today := time.Now().Truncate(24 * time.Hour)

	var usage models.TransactionUsage
	err := s.db.Where("user_id = ? AND date = ?", userID, today).First(&usage).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// create a new usage record
		usage = models.TransactionUsage{
			UserID: userID,
			Date:   today,
			Amount: amount,
		}
		return s.db.Create(&usage).Error
	}

	//update existing usage
	usage.Amount += amount
	return s.db.Save(&usage).Error
}

// getDailyUsage gets total usage for a specific day
func (s *TransactionLimitService) getDailyUsage(userID uuid.UUID, date time.Time) (float64, error) {
	var usage models.TransactionUsage
	err := s.db.Where("user_id = ? AND date = ?", userID, date).First(&usage).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return usage.Amount, nil
}

func (s *TransactionLimitService) getMonthlyUsage(userID uuid.UUID, monthStart time.Time) (float64, error) {
	var result struct {
		total float64
	}
	monthEnd := monthStart.AddDate(0, 1, 0)

	err := s.db.Model(&models.TransactionUsage{}).Select("COALESCE(SUM(amount), 0) as total").Where("user_id = ? AND date >= ? AND date < ?", userID, monthStart, monthEnd).Scan(&result).Error

	if err != nil {
		return 0, err
	}
	return result.total, nil
}

func (s *TransactionLimitService) GetUsageStats(userID uuid.UUID) (map[string]interface{}, error) {
	limit, err := s.GetUserLimIT(userID)
	if err != nil {
		return nil, err
	}

	today := time.Now().Truncate(24 * time.Hour)
	dailyUsage, err := s.getDailyUsage(userID, today)
	if err != nil {
		return nil, err
	}

	monthStart := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.UTC)
	monthlyUsage, err := s.getMonthlyUsage(userID, monthStart)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"limits": map[string]float64{
			"per_transaction": limit.MaxTransactionAmount,
			"daily":           limit.DailyLimit,
			"monthly":         limit.MonthlyLimit,
		},
		"usage": map[string]float64{
			"today":      dailyUsage,
			"this_month": monthlyUsage,
		},
		"remaining": map[string]float64{
			"today":      limit.DailyLimit - dailyUsage,
			"this_month": limit.MonthlyLimit - monthlyUsage,
		},
	}, nil
}
