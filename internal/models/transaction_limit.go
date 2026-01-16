package models

import (
	"time"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TransactionLimit struct {
    ID                   uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    UserID               uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex"`
    User                 User           `gorm:"foreignKey:UserID"`
    MaxTransactionAmount float64        `gorm:"type:decimal(19,4);default:100000"` // Max per transaction
    DailyLimit           float64        `gorm:"type:decimal(19,4);default:500000"` // Max per day
    MonthlyLimit         float64        `gorm:"type:decimal(19,4);default:2000000"` // Max per month
    CreatedAt            time.Time      
    UpdatedAt            time.Time      
    DeletedAt            gorm.DeletedAt `gorm:"index"`
}

func (TransactionLimit) TableName() string {
	return "transaction_limits"
}

type TransactionUsage struct {
    ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    UserID    uuid.UUID      `gorm:"type:uuid;not null;index:idx_user_date"`
    Date      time.Time      `gorm:"not null;index:idx_user_date"`
    Amount    float64        `gorm:"type:decimal(19,4);default:0"`
    CreatedAt time.Time      
    UpdatedAt time.Time      
}

func (TransactionUsage) TableName() string {
	return "transaction_usage"
}