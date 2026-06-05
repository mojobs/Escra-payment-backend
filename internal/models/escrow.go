package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/mojobs/lara-payment-backend.git/pkg/money"
	"gorm.io/gorm"
)

type EscrowOrder struct {
	ID                   uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Reference            string          `gorm:"uniqueIndex;not null;size:60" json:"reference"`
	SellerID             uuid.UUID       `gorm:"type:uuid;not null;index" json:"seller_id"`
	BuyerID              *uuid.UUID      `gorm:"type:uuid;index" json:"buyer_id,omitempty"`
	BuyerName            string          `gorm:"size:120" json:"buyer_name,omitempty"`
	BuyerEmail           string          `gorm:"size:225;index" json:"buyer_email,omitempty"`
	BuyerPhone           string          `gorm:"size:20" json:"buyer_phone,omitempty"`
	SellerWalletID       uuid.UUID       `gorm:"type:uuid;not null;index" json:"seller_wallet_id"`
	BuyerWalletID        *uuid.UUID      `gorm:"type:uuid;index" json:"buyer_wallet_id,omitempty"`
	EscrowWalletID       *uuid.UUID      `gorm:"type:uuid;index" json:"escrow_wallet_id,omitempty"`
	Amount               money.Amount    `gorm:"type:bigint;not null;check:amount > 0" json:"amount_kobo"`
	Currency             string          `gorm:"not null;size:3;default:'NGN'" json:"currency"`
	Title                string          `gorm:"not null;size:150" json:"title"`
	Description          string          `gorm:"type:text" json:"description,omitempty"`
	SalesChannel         string          `gorm:"size:30" json:"sales_channel,omitempty"`
	DeliveryMode         string          `gorm:"size:20;default:'PHYSICAL'" json:"delivery_mode"`
	Status               string          `gorm:"size:40;not null;default:'CREATED';index" json:"status"`
	TrackingReference    string          `gorm:"size:120" json:"tracking_reference,omitempty"`
	DeliveryProofURL     string          `gorm:"size:255" json:"delivery_proof_url,omitempty"`
	BuyerConfirmationTTL int             `gorm:"default:48" json:"buyer_confirmation_ttl_hours"`
	CrossBorder          bool            `gorm:"default:false" json:"cross_border"`
	SettlementCurrency   string          `gorm:"size:3" json:"settlement_currency,omitempty"`
	FXLockedRate         string          `gorm:"size:50" json:"fx_locked_rate,omitempty"`
	FXQuoteReference     string          `gorm:"size:120" json:"fx_quote_reference,omitempty"`
	Metadata             json.RawMessage `gorm:"type:jsonb" json:"metadata,omitempty"`
	DeliveryCodeHash     string          `gorm:"size:255" json:"-"`
	FundedAt             *time.Time      `json:"funded_at,omitempty"`
	ShippedAt            *time.Time      `json:"shipped_at,omitempty"`
	DeliveredAt          *time.Time      `json:"delivered_at,omitempty"`
	ConfirmationDeadline *time.Time      `json:"confirmation_deadline,omitempty"`
	ReleasedAt           *time.Time      `json:"released_at,omitempty"`
	DisputedAt           *time.Time      `json:"disputed_at,omitempty"`
	CancelledAt          *time.Time      `json:"cancelled_at,omitempty"`
	RefundedAt           *time.Time      `json:"refunded_at,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
	DeletedAt            gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (EscrowOrder) TableName() string {
	return "escrow_orders"
}

type EscrowEvent struct {
	ID          uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	EscrowID    uuid.UUID       `gorm:"type:uuid;not null;index" json:"escrow_id"`
	ActorUserID *uuid.UUID      `gorm:"type:uuid;index" json:"actor_user_id,omitempty"`
	Action      string          `gorm:"not null;size:50;index" json:"action"`
	Note        string          `gorm:"type:text" json:"note,omitempty"`
	Metadata    json.RawMessage `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

func (EscrowEvent) TableName() string {
	return "escrow_events"
}

type EscrowDispute struct {
	ID             uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	EscrowID       uuid.UUID      `gorm:"type:uuid;not null;index" json:"escrow_id"`
	RaisedByUserID uuid.UUID      `gorm:"type:uuid;not null;index" json:"raised_by_user_id"`
	Reason         string         `gorm:"not null;size:80" json:"reason"`
	Details        string         `gorm:"type:text" json:"details,omitempty"`
	EvidenceURL    string         `gorm:"size:255" json:"evidence_url,omitempty"`
	Status         string         `gorm:"not null;size:30;default:'OPEN'" json:"status"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	ResolvedAt     *time.Time     `json:"resolved_at,omitempty"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

func (EscrowDispute) TableName() string {
	return "escrow_disputes"
}
