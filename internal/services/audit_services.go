package services

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
)

type AuditService struct {
	db *gorm.DB
}

func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

type AuditLogInput struct {
	UserID     uuid.UUID
	Action     string
	EntityType string
	EntityID   string
	IPAddress  string
	UserAgent  string
	MetaData   map[string]interface{}
	Status     string
	ErrorMsg   string
}

// Log creates an audit log entry
func (s *AuditService) Log(input AuditLogInput) error {
	var metdataJSON string
	if input.MetaData != nil {
		bytes, err := json.Marshal(input.MetaData)
		if err != nil {
			return err
		}
		metdataJSON = string(bytes)
	}

	if input.Status == "" {
		input.Status = "SUCCESS"
	}

	auditLog := models.AuditLog{
		UserID:     input.UserID,
		Action:     input.Action,
		EntityType: input.EntityType,
		EntityID:   input.EntityID,
		IPAddress:  input.IPAddress,
		UserAgent:  input.UserAgent,
		Metadata:   metdataJSON,
		Status:     input.Status,
		ErrorMsg:   input.ErrorMsg,
	}

	return s.db.Create(&auditLog).Error
}

func (s *AuditService) GetUserActivity(userID uuid.UUID, limit int) ([]models.AuditLog, error) {
	var logs []models.AuditLog

	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

func (s *AuditService) CleanUpOldLogs(daysToKeep int) error {
	cutoff := time.Now().AddDate(0, 0, -daysToKeep)
	return s.db.Where("created_at < ?", cutoff).Delete(&models.AuditLog{}).Error
}
