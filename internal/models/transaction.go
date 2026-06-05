package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/mojobs/lara-payment-backend.git/pkg/money"
	"gorm.io/gorm"
)

type Transaction struct {
	ID            uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Reference     string          `gorm:"uniqueIndex;not null;size:50" json:"reference"`
	FromWalletID  *uuid.UUID      `gorm:"type:uuid;index" json:"from_wallet_id,omitempty"`
	ToWalletID    *uuid.UUID      `gorm:"type:uuid;index" json:"to_wallet_id,omitempty"`
	FromWallet    *Wallet         `gorm:"foreignKey:FromWalletID" json:"from_wallet,omitempty"`
	ToWallet      *Wallet         `gorm:"foreignKey:ToWalletID" json:"to_wallet,omitempty"`
	Amount        money.Amount    `gorm:"type:bigint;not null;check:amount > 0" json:"amount_kobo"`
	Currency      string          `gorm:"default:'NGN';size:3" json:"currency"`
	Type          string          `gorm:"size:20;not null" json:"type"`            // TRANSFER, TOP_UP, WITHDRAWAL
	Status        string          `gorm:"default:'PENDING';size:20" json:"status"` // PENDING, COMPLETED, FAILED
	Description   string          `gorm:"type:text" json:"description,omitempty"`
	Metadata      json.RawMessage `gorm:"type:jsonb" json:"metadata,omitempty"`
	LedgerEntries []LedgerEntry   `gorm:"foreignKey:TransactionID" json:"ledger_entries,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	CompletedAt   *time.Time      `json:"completed_at,omitempty"`
	DeletedAt     gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (Transaction) TableName() string {
	return "transactions"
}

type LedgerEntry struct {
	ID            uint         `gorm:"primarykey;autoIncrement" json:"id"`
	TransactionID uuid.UUID    `gorm:"type:uuid;not null;index" json:"transaction_id"`
	Transaction   Transaction  `gorm:"foreignKey:TransactionID" json:"transaction,omitempty"`
	WalletID      uuid.UUID    `gorm:"type:uuid;not null;index" json:"wallet_id"`
	Wallet        Wallet       `gorm:"foreignKey:WalletID" json:"wallet,omitempty"`
	Debit         money.Amount `gorm:"type:bigint;default:0" json:"debit_kobo"`
	Credit        money.Amount `gorm:"type:bigint;default:0" json:"credit_kobo"`
	BalanceAfter  money.Amount `gorm:"type:bigint;not null" json:"balance_after_kobo"`
	CreatedAt     time.Time    `json:"created_at"`
}

func (LedgerEntry) TableName() string {
	return "ledger_entries"
}
