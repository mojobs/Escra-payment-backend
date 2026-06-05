package models

import (
	"github.com/google/uuid"
	"time"
)

type AuditLog struct {
	ID         uint      `gorm:"primarykey;autoIncrement" json:"id"`
	UserID     uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	Action     string    `gorm:"not null;size:100;index" json:"action"`
	EntityType string    `gorm:"size:50" json:"entity_type,omitempty"`
	EntityID   string    `gorm:"type:uuid" json:"entity_id,omitempty"`
	IPAddress  string    `gorm:"size:45" json:"ip_address"`
	UserAgent  string    `gorm:"type:text" json:"user_agent,omitempty"`
	Metadata   string    `gorm:"type:jsonb" json:"metadata,omitempty"`
	Status     string    `gorm:"size:20;default:'SUCCESS'" json:"status"` // SUCCESS, FAILED
	ErrorMsg   string    `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}

// Common Audit actions
const (
	ActionUserLogin         = "USER_LOGIN"
	ActionUserLogout        = "USER_LOGOUT"
	ActionUserRegister      = "USER_REGISTER"
	ActionPinChange         = "PIN_CHANGE"
	ActionTransferInitiated = "TRANSFER_INITIATED"
	ActionTransferCompleted = "TRANSFER_COMPLETED"
	ActionTransferFailed    = "TRANSFER_FAILED"
	ActionBalanceViewed     = "BALANCE_VIEWED"
	ActionProfileViewed     = "PROFILE_VIEWED"
	ActionProfileUpdated    = "PROFILE_UPDATED"
)
