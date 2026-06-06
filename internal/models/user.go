package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID                  uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Phone               string          `gorm:"uniqueIndex;not null;size:20" json:"phone"`
	Password            string          `gorm:"not null;size:100" json:"-"`
	PinHash             string          `gorm:"not null;size:225" json:"-"`
	FirstName           string          `gorm:"size:100" json:"first_name,omitempty"`
	LastName            string          `gorm:"size:100" json:"last_name,omitempty"`
	Email               *string         `gorm:"uniqueIndex;size:225" json:"email,omitempty"`
	Status              string          `gorm:"type:varchar(20);default:'ACTIVE'" json:"status"`
	KYCStatus           string          `gorm:"type:varchar(30);default:'UNVERIFIED';index" json:"kyc_status"`
	KYCProvider         string          `gorm:"size:30" json:"kyc_provider,omitempty"`
	KYCReference        string          `gorm:"size:120;index" json:"kyc_reference,omitempty"`
	KYCIDType           string          `gorm:"size:20" json:"kyc_id_type,omitempty"`
	KYCIDLast4          string          `gorm:"size:4" json:"kyc_id_last4,omitempty"`
	KYCResponse         json.RawMessage `gorm:"type:jsonb" json:"-"`
	KYCVerifiedAt       *time.Time      `json:"kyc_verified_at,omitempty"`
	Wallet              Wallet          `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;" json:"wallet,omitempty"`
	FailedLoginAttempts int             `gorm:"default:0" json:"-"`
	LastFailedLoginAt   *time.Time      `json:"-"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
	DeletedAt           gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (User) TableName() string {
	return "users"
}
