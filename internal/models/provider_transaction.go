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
