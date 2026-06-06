package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/mojobs/lara-payment-backend.git/pkg/money"
	"gorm.io/gorm"
)

type ProviderTransaction struct {
	ID                uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Provider          string          `gorm:"not null;size:30;uniqueIndex:idx_provider_reference" json:"provider"`
	Reference         string          `gorm:"not null;size:80;uniqueIndex:idx_provider_reference" json:"reference"`
	ExternalReference string          `gorm:"size:120;index" json:"external_reference,omitempty"`
	UserID            *uuid.UUID      `gorm:"type:uuid;index" json:"user_id,omitempty"`
	EscrowOrderID     *uuid.UUID      `gorm:"type:uuid;index" json:"escrow_order_id,omitempty"`
	TransactionID     *uuid.UUID      `gorm:"type:uuid;index" json:"transaction_id,omitempty"`
	Type              string          `gorm:"not null;size:30" json:"type"`
	Status            string          `gorm:"not null;size:30;default:'PENDING'" json:"status"`
	Amount            money.Amount    `gorm:"type:bigint;default:0" json:"amount_kobo"`
	AmountText        string          `gorm:"size:80" json:"amount,omitempty"`
	Currency          string          `gorm:"size:10" json:"currency"`
	Fee               money.Amount    `gorm:"type:bigint;default:0" json:"fee_kobo"`
	RequestPayload    json.RawMessage `gorm:"type:jsonb" json:"request_payload,omitempty"`
	ResponsePayload   json.RawMessage `gorm:"type:jsonb" json:"response_payload,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	DeletedAt         gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (ProviderTransaction) TableName() string {
	return "provider_transactions"
}

type WebhookEvent struct {
	ID        uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Provider  string          `gorm:"not null;size:30;uniqueIndex:idx_webhook_event" json:"provider"`
	EventID   string          `gorm:"not null;size:120;uniqueIndex:idx_webhook_event" json:"event_id"`
	EventType string          `gorm:"not null;size:100;index" json:"event_type"`
	Reference string          `gorm:"size:120;index" json:"reference,omitempty"`
	Status    string          `gorm:"not null;size:30;default:'RECEIVED'" json:"status"`
	Payload   json.RawMessage `gorm:"type:jsonb;not null" json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	DeletedAt gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (WebhookEvent) TableName() string {
	return "webhook_events"
}

type KoraVirtualAccount struct {
	ID                 uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID             uuid.UUID       `gorm:"type:uuid;not null;index" json:"user_id"`
	EscrowOrderID      *uuid.UUID      `gorm:"type:uuid;index" json:"escrow_order_id,omitempty"`
	ProviderTxID       *uuid.UUID      `gorm:"type:uuid;index" json:"provider_transaction_id,omitempty"`
	AccountReference   string          `gorm:"not null;uniqueIndex;size:80" json:"account_reference"`
	AccountName        string          `gorm:"size:120" json:"account_name"`
	AccountNumber      string          `gorm:"size:40;index" json:"account_number"`
	BankCode           string          `gorm:"size:20" json:"bank_code"`
	BankName           string          `gorm:"size:120" json:"bank_name"`
	Currency           string          `gorm:"size:3;default:'NGN'" json:"currency"`
	Status             string          `gorm:"size:30;default:'PENDING';index" json:"status"`
	Permanent          bool            `gorm:"default:false" json:"permanent"`
	ProviderCustomerID string          `gorm:"size:120" json:"provider_customer_id,omitempty"`
	RequestPayload     json.RawMessage `gorm:"type:jsonb" json:"request_payload,omitempty"`
	ResponsePayload    json.RawMessage `gorm:"type:jsonb" json:"response_payload,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	DeletedAt          gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (KoraVirtualAccount) TableName() string {
	return "kora_virtual_accounts"
}
