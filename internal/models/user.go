package models

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID                    uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Phone                 string          `gorm:"uniqueIndex;not null;size:20" json:"phone"`
	Password              string          `gorm:"not null;size:100" json:"-"`
	PinHash               string          `gorm:"not null;size:225" json:"-"`
	FirstName             string          `gorm:"size:100" json:"first_name,omitempty"`
	LastName              string          `gorm:"size:100" json:"last_name,omitempty"`
	Email                 *string         `gorm:"uniqueIndex;size:225" json:"email,omitempty"`
	Role                  string          `gorm:"type:varchar(20);default:'buyer';index" json:"role"`
	Status                string          `gorm:"type:varchar(20);default:'ACTIVE'" json:"status"`
	TrustScore            int             `gorm:"default:0" json:"trust_score"`
	Rating                float64         `gorm:"default:0" json:"rating"`
	BusinessName          string          `gorm:"size:160" json:"business_name,omitempty"`
	BusinessType          string          `gorm:"size:120" json:"business_type,omitempty"`
	RCNumber              string          `gorm:"size:60" json:"rc_number,omitempty"`
	Website               string          `gorm:"size:255" json:"website,omitempty"`
	BusinessAddress       string          `gorm:"size:255" json:"address,omitempty"`
	BusinessCity          string          `gorm:"size:100" json:"city,omitempty"`
	BusinessCountry       string          `gorm:"size:100" json:"country,omitempty"`
	BusinessSupportPhone  string          `gorm:"size:20" json:"support_phone,omitempty"`
	KYCStatus             string          `gorm:"type:varchar(30);default:'UNVERIFIED';index" json:"kyc_status"`
	KYCProvider           string          `gorm:"size:30" json:"kyc_provider,omitempty"`
	KYCReference          string          `gorm:"size:120;index" json:"kyc_reference,omitempty"`
	KYCIDType             string          `gorm:"size:20" json:"kyc_id_type,omitempty"`
	KYCIDLast4            string          `gorm:"size:4" json:"kyc_id_last4,omitempty"`
	KYCResponse           json.RawMessage `gorm:"type:jsonb" json:"-"`
	KYCVerifiedAt         *time.Time      `json:"kyc_verified_at,omitempty"`
	KYCDocumentType       string          `gorm:"size:40" json:"kyc_document_type,omitempty"`
	KYCDocumentURL        string          `gorm:"size:500" json:"kyc_document_url,omitempty"`
	KYCDocumentUploadedAt *time.Time      `json:"kyc_document_uploaded_at,omitempty"`
	KYCRejectionReason    string          `gorm:"size:500" json:"kyc_rejection_reason,omitempty"`
	Wallet                Wallet          `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;" json:"wallet,omitempty"`
	FailedLoginAttempts   int             `gorm:"default:0" json:"-"`
	LastFailedLoginAt     *time.Time      `json:"-"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
	DeletedAt             gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (User) TableName() string {
	return "users"
}

func NormalizeUserRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "seller":
		return "seller"
	default:
		return "buyer"
	}
}

func IsValidUserRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "", "buyer", "seller":
		return true
	default:
		return false
	}
}

func NormalizeKYCDocumentType(documentType string) string {
	return strings.ToUpper(strings.TrimSpace(documentType))
}

func IsValidKYCDocumentType(documentType string) bool {
	switch NormalizeKYCDocumentType(documentType) {
	case "CAC_CERTIFICATE", "PASSPORT":
		return true
	default:
		return false
	}
}
