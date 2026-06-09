package services

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
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

		response, err := decodeRawJSON(idempotency.Response)
		if err != nil {
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

	reqJSON, err := jsonForDB(requestBody)
	if err != nil {
		return nil, err
	}

	idempotency = models.IdempotencyKey{
		Key:         key,
		UserID:      userID,
		Endpoint:    endpoint,
		RequestHash: requestHash,
		RequestBody: reqJSON,
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
	resJSON, err := jsonForDB(response)
	if err != nil {
		return err
	}

	return s.db.Model(&models.IdempotencyKey{}).
		Where("key = ? AND user_id = ? AND endpoint = ?", key, userID, endpoint).
		Updates(map[string]interface{}{
			"response":    nullableRawJSON(resJSON),
			"status_code": statusCode,
			"state":       "COMPLETED",
		}).Error
}

func (s *IdempotencyService) StoreFailure(key string, userID uuid.UUID, endpoint string, response interface{}, statusCode int) error {
	resJSON, err := jsonForDB(response)
	if err != nil {
		return err
	}

	return s.db.Model(&models.IdempotencyKey{}).
		Where("key = ? AND user_id = ? AND endpoint = ?", key, userID, endpoint).
		Updates(map[string]interface{}{
			"response":    nullableRawJSON(resJSON),
			"status_code": statusCode,
			"state":       "FAILED",
		}).Error
}

func (s *IdempotencyService) CleanupExpired() error {
	return s.db.Where("expires_at < ?", time.Now()).Delete(&models.IdempotencyKey{}).Error
}

func jsonForDB(value interface{}) (json.RawMessage, error) {
	if value == nil {
		return nil, nil
	}

	switch typed := value.(type) {
	case json.RawMessage:
		return validRawJSON(typed)
	case []byte:
		return validRawJSON(typed)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil, nil
		}
		if json.Valid([]byte(trimmed)) {
			return json.RawMessage(append([]byte(nil), trimmed...)), nil
		}
		encoded, err := json.Marshal(trimmed)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(encoded), nil
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return validRawJSON(encoded)
	}
}

func validRawJSON(payload []byte) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if !json.Valid(trimmed) {
		return nil, errors.New("request or response payload is not valid JSON")
	}
	return json.RawMessage(append([]byte(nil), trimmed...)), nil
}

func nullableRawJSON(payload json.RawMessage) interface{} {
	if len(payload) == 0 {
		return nil
	}
	return payload
}

func decodeRawJSON(payload json.RawMessage) (interface{}, error) {
	if len(payload) == 0 {
		return nil, nil
	}

	var value interface{}
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, err
	}
	return value, nil
}
