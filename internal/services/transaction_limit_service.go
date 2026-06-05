package services

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/mojobs/lara-payment-backend.git/pkg/money"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
	return s.createDefaultLimit(s.db, userID)
}

func (s *TransactionLimitService) createDefaultLimit(tx *gorm.DB, userID uuid.UUID) error {
	limit := models.TransactionLimit{
		UserID:               userID,
		MaxTransactionAmount: money.FromMinorUnits(10000000),
		DailyLimit:           money.FromMinorUnits(50000000),
		MonthlyLimit:         money.FromMinorUnits(200000000),
	}

	return tx.Create(&limit).Error
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

func (s *TransactionLimitService) CheckTransactionLimit(userID uuid.UUID, amount money.Amount) error {
	limit, err := s.GetUserLimIT(userID)
	if err != nil {
		return err
	}

	//Check per transaction
	if amount > limit.MaxTransactionAmount {
		return errors.New("amount exceeds maximum limit")
	}

	//Check daily limit
	today := dayStart(time.Now().UTC())
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

func (s *TransactionLimitService) CheckAndRecordTransaction(tx *gorm.DB, userID uuid.UUID, amount money.Amount) error {
	var limit models.TransactionLimit
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		First(&limit).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := s.createDefaultLimit(tx, userID); err != nil {
				return err
			}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("user_id = ?", userID).
				First(&limit).Error; err != nil {
				return err
			}
		} else {
			return err
		}
	}

	if amount > limit.MaxTransactionAmount {
		return errors.New("amount exceeds maximum limit")
	}

	now := time.Now().UTC()
	today := dayStart(now)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	var usages []models.TransactionUsage
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND date >= ? AND date < ?", userID, monthStart, monthEnd).
		Find(&usages).Error; err != nil {
		return err
	}

	var dailyUsage money.Amount
	var monthlyUsage money.Amount
	var todayUsage *models.TransactionUsage
	for i := range usages {
		monthlyUsage += usages[i].Amount
		if usages[i].Date.Equal(today) {
			dailyUsage = usages[i].Amount
			todayUsage = &usages[i]
		}
	}

	if dailyUsage+amount > limit.DailyLimit {
		return errors.New("transaction would exceed daily limit")
	}

	if monthlyUsage+amount > limit.MonthlyLimit {
		return errors.New("transaction would exceed monthly limit")
	}

	if todayUsage == nil {
		usage := models.TransactionUsage{
			UserID: userID,
			Date:   today,
			Amount: amount,
		}
		return tx.Create(&usage).Error
	}

	return tx.Model(todayUsage).Update("amount", todayUsage.Amount+amount).Error
}

// RecordTransaction records a successful transaction for limit tracking
func (s *TransactionLimitService) RecordTransaction(userID uuid.UUID, amount money.Amount) error {
	today := dayStart(time.Now().UTC())

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
func (s *TransactionLimitService) getDailyUsage(userID uuid.UUID, date time.Time) (money.Amount, error) {
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

func (s *TransactionLimitService) getMonthlyUsage(userID uuid.UUID, monthStart time.Time) (money.Amount, error) {
	var total money.Amount
	monthEnd := monthStart.AddDate(0, 1, 0)

	row := s.db.Model(&models.TransactionUsage{}).Select("COALESCE(SUM(amount), 0) as total").Where("user_id = ? AND date >= ? AND date < ?", userID, monthStart, monthEnd).Row()

	if err := row.Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *TransactionLimitService) GetUsageStats(userID uuid.UUID) (map[string]interface{}, error) {
	limit, err := s.GetUserLimIT(userID)
	if err != nil {
		return nil, err
	}

	today := dayStart(time.Now().UTC())
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
		"limits": map[string]money.Amount{
			"per_transaction": limit.MaxTransactionAmount,
			"daily":           limit.DailyLimit,
			"monthly":         limit.MonthlyLimit,
		},
		"usage": map[string]money.Amount{
			"today":      dailyUsage,
			"this_month": monthlyUsage,
		},
		"remaining": map[string]money.Amount{
			"today":      limit.DailyLimit - dailyUsage,
			"this_month": limit.MonthlyLimit - monthlyUsage,
		},
	}, nil
}

func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
