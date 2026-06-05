package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/mojobs/lara-payment-backend.git/pkg/money"
	"gorm.io/gorm"
)

type TransactionLimit struct {
	ID                   uuid.UUID    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID               uuid.UUID    `gorm:"type:uuid;not null;uniqueIndex"`
	User                 User         `gorm:"foreignKey:UserID"`
	MaxTransactionAmount money.Amount `gorm:"type:bigint;default:10000000"`  // Max per transaction: NGN 100,000
	DailyLimit           money.Amount `gorm:"type:bigint;default:50000000"`  // Max per day: NGN 500,000
	MonthlyLimit         money.Amount `gorm:"type:bigint;default:200000000"` // Max per month: NGN 2,000,000
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedAt            gorm.DeletedAt `gorm:"index"`
}

func (TransactionLimit) TableName() string {
	return "transaction_limits"
}

type TransactionUsage struct {
	ID        uuid.UUID    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID    `gorm:"type:uuid;not null;uniqueIndex:idx_user_date"`
	Date      time.Time    `gorm:"not null;uniqueIndex:idx_user_date"`
	Amount    money.Amount `gorm:"type:bigint;default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (TransactionUsage) TableName() string {
	return "transaction_usage"
}
