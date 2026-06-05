package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/services"
)

// responseWriter wraps gin.ResponseWriter to capture response
type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func IdempotencyMiddleware(idempotencyService *services.IdempotencyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only apply to POST requests
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}

		//Get idempotency key
		idempotencyKey := c.GetHeader("Idempotency-Key")
		if idempotencyKey == "" {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "missing_idempotency_key",
				Message: "Idempotency-Key header is required for POST requests",
			})
			c.Abort()
			return
		}

		//Get user ID from JWT
		userIDStr := c.GetString("user_id")
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "invalid_user_id",
				Message: "Invalid user ID",
			})
			c.Abort()
			return
		}

		endpoint := c.Request.URL.Path

		//Read request body before reserving the key so concurrent retries race on the same request hash.
		var requestBody interface{}
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			_ = json.Unmarshal(bodyBytes, &requestBody)
		}
		requestHash := services.RequestHash(bodyBytes)

		// Reserve or replay the key.
		result, err := idempotencyService.ReserveKey(idempotencyKey, userID, endpoint, requestBody, requestHash)
		if err != nil {
			c.JSON(http.StatusConflict, models.ErrorResponse{
				Error:   "idempotency_check_failed",
				Message: err.Error(),
			})
			c.Abort()
			return
		}

		//If key exists, return cached response
		if result.Exists {
			c.JSON(result.StatusCode, result.Response)
			c.Abort()
			return
		}

		//Wrap response writer to capture response
		writer := &responseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
		}
		c.Writer = writer

		c.Next()

		//Store idempotency key with response (only for successful request)

		var response interface{}
		_ = json.Unmarshal(writer.body.Bytes(), &response)

		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			idempotencyService.StoreKey(idempotencyKey, userID, endpoint, requestBody, response, c.Writer.Status())
			return
		}

		idempotencyService.StoreFailure(idempotencyKey, userID, endpoint, response, c.Writer.Status())
	}
}
