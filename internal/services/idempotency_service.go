package services

import (
	"crypto/sha256"
	"encoding/hex"
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

func RequestHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func (s *IdempotencyService) ReserveKey(key string, userID uuid.UUID, endpoint string, requestBody interface{}, requestHash string) (*IdempotencyResult, error) {
	var idempotency models.IdempotencyKey

	err := s.db.Where("key = ? AND user_id = ? AND endpoint = ? AND expires_at > ?",
		key, userID, endpoint, time.Now()).First(&idempotency).Error

	if err == nil {
		if idempotency.RequestHash != requestHash {
			return nil, errors.New("idempotency key reused with a different request body")
		}

		if idempotency.State == "PENDING" {
			return nil, errors.New("idempotent request is already in progress")
		}

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

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	reqJSON, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	idempotency = models.IdempotencyKey{
		Key:         key,
		UserID:      userID,
		Endpoint:    endpoint,
		RequestHash: requestHash,
		RequestBody: string(reqJSON),
		State:       "PENDING",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}

	if err := s.db.Create(&idempotency).Error; err != nil {
		return nil, err
	}

	return &IdempotencyResult{Exists: false}, nil
}

//StoreKey stores a new idempotency key with its response

func (s *IdempotencyService) StoreKey(key string, userID uuid.UUID, endpoint string, requestBody, response interface{}, statusCode int) error {
	resJSON, err := json.Marshal(response)
	if err != nil {
		return err
	}

	return s.db.Model(&models.IdempotencyKey{}).
		Where("key = ? AND user_id = ? AND endpoint = ?", key, userID, endpoint).
		Updates(map[string]interface{}{
			"response":    string(resJSON),
			"status_code": statusCode,
			"state":       "COMPLETED",
		}).Error
}

func (s *IdempotencyService) StoreFailure(key string, userID uuid.UUID, endpoint string, response interface{}, statusCode int) error {
	resJSON, err := json.Marshal(response)
	if err != nil {
		return err
	}

	return s.db.Model(&models.IdempotencyKey{}).
		Where("key = ? AND user_id = ? AND endpoint = ?", key, userID, endpoint).
		Updates(map[string]interface{}{
			"response":    string(resJSON),
			"status_code": statusCode,
			"state":       "FAILED",
		}).Error
}

func (s *IdempotencyService) CleanupExpired() error {
	return s.db.Where("expires_at < ?", time.Now()).Delete(&models.IdempotencyKey{}).Error
}
