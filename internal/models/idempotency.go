package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type IdempotencyKey struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Key         string    `gorm:"uniqueIndex;not null; size:225"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index"`
	Endpoint    string    `gorm:"not null;size:255"`
	RequestBody string    `gorm:"type:jsonb"`
	Response    string    `gorm:"type:jsonb"`
	StatusCode  int       `gorm:"not null"`
	CreatedAt   time.Time
	ExpiresAt   time.Time      `gorm:"index"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}


func (IdempotencyKey) TableName() string {
	return "idempotency_keys"
}