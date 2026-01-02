package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type User struct {
	ID                  uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Phone               string         `gorm:"uniqueIndex;not null;size:20" json:"phone"`
	Password            string         `gorm:"not null;size:100" json:"password"`
	PinHash             string         `gorm:"not null;size:225" json:"-"`
	FirstName           string         `gorm:"size:100" json:"first_name,omitempty"`
	LastName            string         `gorm:"size:100" json:"last_name,omitempty"`
	Email               string         `gorm:"uniqueIndex;size:225" json:"email,omitempty"`
	Status              string         `gorm:"type:varchar(20);default:'ACTIVE'" json:"status"`
	WalletID            uuid.UUID       `gorm:"type:uuid;not null" json:"wallet_id"`
	FailedLoginAttempts int            `gorm:"default:0" json:"-"`
	LastFailedLoginAt   *time.Time     `json:"-"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string {
	return "users"
}
