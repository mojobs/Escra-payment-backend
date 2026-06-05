package models

import (
	"time"

	"github.com/mojobs/lara-payment-backend.git/pkg/money"
)

type RegisterRequest struct {
	Phone     string `json:"phone" binding:"required,min=10,max=15"`
	Password  string `json:"password" binding:"required,min=8,max=15"`
	FirstName string `json:"first_name" binding:"required,min=2,max=100"`
	LastName  string `json:"last_name" binding:"required,min=2,max=100"`
	Pin       string `json:"pin" binding:"required,min=4,max=6,numeric"`
	Email     string `json:"email" binding:"omitempty,email"`
}

type LoginRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required,min=8,max=15"`
	Pin      string `json:"pin,omitempty" binding:"omitempty,min=4,max=6,numeric"`
}

type AuthResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
}

type UserResponse struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	Email     string `json:"email,omitempty"`
	Status    string `json:"status" binding:"omitempty,oneof=ACTIVE INACTIVE BLOCKED"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type SuccessResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type TransferRequest struct {
	RecipientWalletID string       `json:"wallet_id" binding:"required,uuid"`
	Amount            money.Amount `json:"amount_kobo" binding:"required"`
	Description       string       `json:"description" binding:"omitempty,max=500"`
	Pin               string       `json:"pin" binding:"required,min=4,max=6,numeric"`
}

type TransferResponse struct {
	Success     bool              `json:"success"`
	Transaction TransactionDetail `json:"transaction"`
	NewBalance  money.Amount      `json:"new_balance_kobo"`
}

type TransactionDetail struct {
	ID        string       `json:"id"`
	Reference string       `json:"reference"`
	Amount    money.Amount `json:"amount_kobo"`
	Recipient Recipient    `json:"recipient"`
	Status    string       `json:"status"`
	CreatedAt string       `json:"created_at"`
}

type Recipient struct {
	WalletID string `json:"wallet_id"`
	Name     string `json:"name"`
}

type TransactionHistoryResponse struct {
	ID          string       `json:"id"`
	Reference   string       `json:"reference"`
	Amount      money.Amount `json:"amount_kobo"`
	Type        string       `json:"type"` // DEBIT or CREDIT
	Status      string       `json:"status"`
	Description string       `json:"description,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
}

type KoraBankPayoutRequest struct {
	BankCode      string       `json:"bank_code" binding:"required"`
	AccountNumber string       `json:"account_number" binding:"required,min=10,max=34"`
	AccountName   string       `json:"account_name" binding:"omitempty,max=120"`
	CustomerEmail string       `json:"customer_email" binding:"omitempty,email"`
	Amount        money.Amount `json:"amount_kobo" binding:"required"`
	Currency      string       `json:"currency" binding:"omitempty,len=3"`
	Narration     string       `json:"narration" binding:"omitempty,max=120"`
	Pin           string       `json:"pin" binding:"required,min=4,max=6,numeric"`
}

type KoraBankResolveResponse struct {
	BankName      string `json:"bank_name"`
	BankCode      string `json:"bank_code"`
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
}

type ProviderTransactionResponse struct {
	ID                string       `json:"id"`
	Provider          string       `json:"provider"`
	Reference         string       `json:"reference"`
	ExternalReference string       `json:"external_reference,omitempty"`
	Type              string       `json:"type"`
	Status            string       `json:"status"`
	Amount            money.Amount `json:"amount_kobo,omitempty"`
	AmountText        string       `json:"amount,omitempty"`
	Currency          string       `json:"currency"`
	CreatedAt         time.Time    `json:"created_at"`
}

type QuidaxCryptoWithdrawalRequest struct {
	Currency        string `json:"currency" binding:"required"`
	Amount          string `json:"amount" binding:"required"`
	FundUID         string `json:"fund_uid" binding:"required"`
	FundUID2        string `json:"fund_uid2,omitempty"`
	Network         string `json:"network,omitempty"`
	TransactionNote string `json:"transaction_note,omitempty" binding:"omitempty,max=120"`
	Narration       string `json:"narration,omitempty" binding:"omitempty,max=120"`
	ProviderUserID  string `json:"provider_user_id,omitempty"`
	Pin             string `json:"pin" binding:"required,min=4,max=6,numeric"`
}

type CreateEscrowOrderRequest struct {
	Title                     string       `json:"title" binding:"required,min=3,max=150"`
	Description               string       `json:"description" binding:"omitempty,max=1000"`
	Amount                    money.Amount `json:"amount_kobo" binding:"required"`
	Currency                  string       `json:"currency" binding:"omitempty,len=3"`
	SalesChannel              string       `json:"sales_channel" binding:"omitempty,oneof=WHATSAPP INSTAGRAM TELEGRAM WEBSITE OTHER"`
	DeliveryMode              string       `json:"delivery_mode" binding:"omitempty,oneof=PHYSICAL DIGITAL SERVICE"`
	BuyerConfirmationTTLHours int          `json:"buyer_confirmation_ttl_hours" binding:"omitempty,min=1,max=168"`
	CrossBorder               bool         `json:"cross_border"`
	SettlementCurrency        string       `json:"settlement_currency,omitempty" binding:"omitempty,len=3"`
	FXLockedRate              string       `json:"fx_locked_rate,omitempty" binding:"omitempty,max=50"`
	FXQuoteReference          string       `json:"fx_quote_reference,omitempty" binding:"omitempty,max=120"`
	Metadata                  interface{}  `json:"metadata,omitempty"`
}

type KoraEscrowCheckoutRequest struct {
	RedirectURL       string   `json:"redirect_url" binding:"omitempty,url,max=255"`
	NotificationURL   string   `json:"notification_url" binding:"omitempty,url,max=255"`
	Channels          []string `json:"channels,omitempty"`
	DefaultChannel    string   `json:"default_channel,omitempty" binding:"omitempty,max=30"`
	MerchantBearsCost bool     `json:"merchant_bears_cost"`
}

type FundEscrowOrderRequest struct {
	Pin string `json:"pin" binding:"required,min=4,max=6,numeric"`
}

type ShipEscrowOrderRequest struct {
	TrackingReference string `json:"tracking_reference" binding:"omitempty,max=120"`
	DeliveryProofURL  string `json:"delivery_proof_url" binding:"omitempty,max=255,url"`
	Note              string `json:"note" binding:"omitempty,max=500"`
}

type MarkDeliveredEscrowOrderRequest struct {
	DeliveryProofURL string `json:"delivery_proof_url" binding:"omitempty,max=255,url"`
	Note             string `json:"note" binding:"omitempty,max=500"`
}

type ConfirmEscrowDeliveryRequest struct {
	DeliveryCode string `json:"delivery_code,omitempty" binding:"omitempty,min=6,max=6,numeric"`
	Pin          string `json:"pin,omitempty" binding:"omitempty,min=4,max=6,numeric"`
}

type OpenEscrowDisputeRequest struct {
	Reason      string `json:"reason" binding:"required,min=3,max=80"`
	Details     string `json:"details" binding:"omitempty,max=1000"`
	EvidenceURL string `json:"evidence_url" binding:"omitempty,max=255,url"`
}

type CancelEscrowOrderRequest struct {
	Reason string `json:"reason" binding:"omitempty,max=500"`
}

type RefundEscrowOrderRequest struct {
	Reason string `json:"reason" binding:"omitempty,max=500"`
}

type ResolveEscrowDisputeRequest struct {
	Action         string `json:"action" binding:"required,oneof=RELEASE REFUND"`
	ResolutionNote string `json:"resolution_note" binding:"omitempty,max=500"`
}

type EscrowOrderResponse struct {
	ID                   string             `json:"id"`
	Reference            string             `json:"reference"`
	SellerID             string             `json:"seller_id"`
	BuyerID              string             `json:"buyer_id,omitempty"`
	BuyerName            string             `json:"buyer_name,omitempty"`
	BuyerEmail           string             `json:"buyer_email,omitempty"`
	BuyerPhone           string             `json:"buyer_phone,omitempty"`
	Amount               money.Amount       `json:"amount_kobo"`
	Currency             string             `json:"currency"`
	Title                string             `json:"title"`
	Description          string             `json:"description,omitempty"`
	SalesChannel         string             `json:"sales_channel,omitempty"`
	DeliveryMode         string             `json:"delivery_mode"`
	Status               string             `json:"status"`
	TrackingReference    string             `json:"tracking_reference,omitempty"`
	DeliveryProofURL     string             `json:"delivery_proof_url,omitempty"`
	BuyerConfirmationTTL int                `json:"buyer_confirmation_ttl_hours"`
	CrossBorder          bool               `json:"cross_border"`
	SettlementCurrency   string             `json:"settlement_currency,omitempty"`
	FXLockedRate         string             `json:"fx_locked_rate,omitempty"`
	FXQuoteReference     string             `json:"fx_quote_reference,omitempty"`
	FundedAt             *time.Time         `json:"funded_at,omitempty"`
	ShippedAt            *time.Time         `json:"shipped_at,omitempty"`
	DeliveredAt          *time.Time         `json:"delivered_at,omitempty"`
	ConfirmationDeadline *time.Time         `json:"confirmation_deadline,omitempty"`
	ReleasedAt           *time.Time         `json:"released_at,omitempty"`
	DisputedAt           *time.Time         `json:"disputed_at,omitempty"`
	CancelledAt          *time.Time         `json:"cancelled_at,omitempty"`
	RefundedAt           *time.Time         `json:"refunded_at,omitempty"`
	CreatedAt            time.Time          `json:"created_at"`
	UpdatedAt            time.Time          `json:"updated_at"`
	Events               []EscrowEventDTO   `json:"events,omitempty"`
	Disputes             []EscrowDisputeDTO `json:"disputes,omitempty"`
}

type EscrowEventDTO struct {
	Action    string    `json:"action"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type EscrowDisputeDTO struct {
	ID          string     `json:"id"`
	Reason      string     `json:"reason"`
	Details     string     `json:"details,omitempty"`
	EvidenceURL string     `json:"evidence_url,omitempty"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

type FundEscrowOrderResponse struct {
	Order        EscrowOrderResponse `json:"order"`
	DeliveryCode string              `json:"delivery_code"`
}

type EscrowPublicOrderResponse struct {
	Reference            string       `json:"reference"`
	SellerName           string       `json:"seller_name"`
	Title                string       `json:"title"`
	Description          string       `json:"description,omitempty"`
	Amount               money.Amount `json:"amount_kobo"`
	Currency             string       `json:"currency"`
	SalesChannel         string       `json:"sales_channel,omitempty"`
	DeliveryMode         string       `json:"delivery_mode"`
	Status               string       `json:"status"`
	BuyerConfirmationTTL int          `json:"buyer_confirmation_ttl_hours"`
	CrossBorder          bool         `json:"cross_border"`
	SettlementCurrency   string       `json:"settlement_currency,omitempty"`
	FXLockedRate         string       `json:"fx_locked_rate,omitempty"`
	FXQuoteReference     string       `json:"fx_quote_reference,omitempty"`
	CreatedAt            time.Time    `json:"created_at"`
}

type EscrowCheckoutResponse struct {
	Order               EscrowOrderResponse         `json:"order"`
	Provider            string                      `json:"provider"`
	ProviderReference   string                      `json:"provider_reference"`
	CheckoutURL         string                      `json:"checkout_url"`
	CheckoutReference   string                      `json:"checkout_reference"`
	CheckoutStatus      string                      `json:"checkout_status"`
	RedirectURL         string                      `json:"redirect_url,omitempty"`
	NotificationURL     string                      `json:"notification_url,omitempty"`
	MerchantBearsCost   bool                        `json:"merchant_bears_cost"`
	DefaultChannel      string                      `json:"default_channel,omitempty"`
	Channels            []string                    `json:"channels,omitempty"`
	ProviderTransaction ProviderTransactionResponse `json:"provider_transaction"`
}
