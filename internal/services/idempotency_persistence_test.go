//go:build cgo
// +build cgo

package services

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
)

func TestIdempotencyStoresValidJSONForPendingAndCompletedRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.IdempotencyKey{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	service := NewIdempotencyService(db)
	userID := uuid.New()
	key := uuid.NewString()
	endpoint := "/api/v1/escrows/orders"

	result, err := service.ReserveKey(key, userID, endpoint, "", RequestHash(nil))
	if err != nil {
		t.Fatalf("reserve key: %v", err)
	}
	if result.Exists {
		t.Fatalf("new key should not exist")
	}

	var pending models.IdempotencyKey
	if err := db.Where("key = ?", key).First(&pending).Error; err != nil {
		t.Fatalf("get pending row: %v", err)
	}
	if !json.Valid([]byte(pending.RequestBody)) {
		t.Fatalf("request body is not valid json: %q", pending.RequestBody)
	}
	if pending.Response != "null" {
		t.Fatalf("pending response should be JSON null, got %q", pending.Response)
	}

	if err := service.StoreKey(key, userID, endpoint, nil, map[string]interface{}{
		"success": true,
		"count":   1,
	}, 201); err != nil {
		t.Fatalf("store key: %v", err)
	}

	var completed models.IdempotencyKey
	if err := db.Where("key = ?", key).First(&completed).Error; err != nil {
		t.Fatalf("get completed row: %v", err)
	}
	if !json.Valid([]byte(completed.Response)) {
		t.Fatalf("response is not valid json: %q", completed.Response)
	}
	if completed.State != "COMPLETED" {
		t.Fatalf("state = %q", completed.State)
	}
}
