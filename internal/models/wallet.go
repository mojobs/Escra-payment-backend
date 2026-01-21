package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type Wallet struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"wallet_id"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	Balance   float64        `gorm:"type:decimal(19,4);default:0;check:balance >= 0" json:"balance"`
	Currency  string         `gorm:"default:'NGN';size:3" json:"currency"`
	Status    string         `gorm:"default:'ACTIVE';size:20" json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Wallet) TableName() string {
	return "wallets"
}

type BalanceResponse struct {
	Balance  float64 `json:"balance"`
	Currency string  `json:"currency"`
}
