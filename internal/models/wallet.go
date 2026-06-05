package models

import (
	"github.com/google/uuid"
	"github.com/mojobs/lara-payment-backend.git/pkg/money"
	"gorm.io/gorm"
	"time"
)

type Wallet struct {
	ID             uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"wallet_id"`
	UserID         *uuid.UUID     `gorm:"type:uuid;uniqueIndex:idx_wallet_owner" json:"user_id,omitempty"`
	OwnerType      string         `gorm:"not null;size:20;default:'USER';uniqueIndex:idx_wallet_owner" json:"owner_type"`
	OwnerReference string         `gorm:"not null;size:100;uniqueIndex:idx_wallet_owner" json:"owner_reference"`
	Balance        money.Amount   `gorm:"type:bigint;default:0;check:balance >= 0" json:"balance_kobo"`
	Currency       string         `gorm:"default:'NGN';size:3" json:"currency"`
	Status         string         `gorm:"default:'ACTIVE';size:20" json:"status"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Wallet) TableName() string {
	return "wallets"
}

type BalanceResponse struct {
	BalanceKobo      money.Amount `json:"balance_kobo"`
	BalanceFormatted string       `json:"balance"`
	Currency         string       `json:"currency"`
}
