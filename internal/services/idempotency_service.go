package services

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
)

type IdempotencyService struct {
	db *gorm.DB
}

func NewIdempotencyService(db *gorm.DB) *IdempotencyService {
	return &IdempotencyService{db: db}
}

type IdempotencyResult struct {
	Exists     bool
	StatusCode int
	Response   interface{}
}

func (s *IdempotencyService) CheckKey(key string, userID uuid.UUID, endpoint string) (*IdempotencyResult, error) {
	var idempotency models.IdempotencyKey

	err := s.db.Where("key = ? AND user_id = ? AND endpoint = ? AND expires_at > ?",
		key, userID, endpoint, time.Now()).First(&idempotency).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &IdempotencyResult{Exists: false}, nil
		}
		return nil, err
	}

	//Unmarshal response
	var response interface{}
	if err := json.Unmarshal([]byte(idempotency.Response), &response); err != nil {
		return nil, err
	}

	return &IdempotencyResult{
		Exists:     true,
		StatusCode: idempotency.StatusCode,
		Response:   response,
	}, nil
}

//StoreKey stores a new idempotency key with its response

func (s *IdempotencyService) StoreKey(key string, userID uuid.UUID, endpoint string, requestBody, response interface{}, statusCode int) error {
	reqJSON, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	resJSON, err := json.Marshal(response)
	if err != nil {
		return err
	}

	idempotency := models.IdempotencyKey{
		Key:         key,
		UserID:      userID,
		Endpoint:    endpoint,
		RequestBody: string(reqJSON),
		Response:    string(resJSON),
		StatusCode:  statusCode,
		ExpiresAt:   time.Now().Add(24 * time.Hour), //Keys expire after 24 hours
	}

	return s.db.Create(&idempotency).Error

}

func (s *IdempotencyService) CleanupExpired() error {
	return s.db.Where("expires_at < ?", time.Now()).Delete(&models.IdempotencyKey{}).Error
}
