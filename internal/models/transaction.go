package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type Transaction struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Reference     string         `gorm:"uniqueIndex;not null;size:50" json:"reference"`
	FromWalletID  *uuid.UUID     `gorm:"type:uuid;index" json:"from_wallet_id,omitempty"`
	ToWalletID    *uuid.UUID     `gorm:"type:uuid;index" json:"to_wallet_id,omitempty"`
	FromWallet    *Wallet        `gorm:"foreignKey:FromWalletID" json:"from_wallet,omitempty"`
	ToWallet      *Wallet        `gorm:"foreignKey:ToWalletID" json:"to_wallet,omitempty"`
	Amount        float64        `gorm:"type:decimal(19,4);not null;check:amount > 0" json:"amount"`
	Currency      string         `gorm:"default:'NGN';size:3" json:"currency"`
	Type          string         `gorm:"size:20;not null" json:"type"`            // TRANSFER, TOP_UP, WITHDRAWAL
	Status        string         `gorm:"default:'PENDING';size:20" json:"status"` // PENDING, COMPLETED, FAILED
	Description   string         `gorm:"type:text" json:"description,omitempty"`
	Metadata      string         `gorm:"type:jsonb" json:"metadata,omitempty"`
	LedgerEntries []LedgerEntry  `gorm:"foreignKey:TransactionID" json:"ledger_entries,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Transaction) TableName() string {
	return "transactions"
}

type LedgerEntry struct {
	ID            uint        `gorm:"primarykey;autoIncrement" json:"id"`
	TransactionID uuid.UUID   `gorm:"type:uuid;not null;index" json:"transaction_id"`
	Transaction   Transaction `gorm:"foreignKey:TransactionID" json:"transaction,omitempty"`
	WalletID      uuid.UUID   `gorm:"type:uuid;not null;index" json:"wallet_id"`
	Wallet        Wallet      `gorm:"foreignKey:WalletID" json:"wallet,omitempty"`
	Debit         float64     `gorm:"type:decimal(19,4);default:0" json:"debit"`
	Credit        float64     `gorm:"type:decimal(19,4);default:0" json:"credit"`
	BalanceAfter  float64     `gorm:"type:decimal(19,4);not null" json:"balance_after"`
	CreatedAt     time.Time   `json:"created_at"`
}

func (LedgerEntry) TableName() string {
	return "ledger_entries"
}
