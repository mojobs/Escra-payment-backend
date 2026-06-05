package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type IdempotencyKey struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Key         string    `gorm:"not null;size:225;uniqueIndex:idx_idempotency_scope"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_idempotency_scope"`
	Endpoint    string    `gorm:"not null;size:255;uniqueIndex:idx_idempotency_scope"`
	RequestHash string    `gorm:"not null;size:64;default:''"`
	RequestBody string    `gorm:"type:jsonb"`
	Response    string    `gorm:"type:jsonb"`
	StatusCode  int       `gorm:"not null;default:0"`
	State       string    `gorm:"not null;size:20;default:'PENDING'"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ExpiresAt   time.Time      `gorm:"index"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (IdempotencyKey) TableName() string {
	return "idempotency_keys"
}
