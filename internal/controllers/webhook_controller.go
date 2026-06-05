package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/services"
)

type WebhookController struct {
	webhookService      *services.WebhookService
	koraWebhookSecret   string
	quidaxWebhookSecret string
}

func NewWebhookController(webhookService *services.WebhookService, koraWebhookSecret, quidaxWebhookSecret string) *WebhookController {
	return &WebhookController{
		webhookService:      webhookService,
		koraWebhookSecret:   koraWebhookSecret,
		quidaxWebhookSecret: quidaxWebhookSecret,
	}
}

func (ctrl *WebhookController) Kora(c *gin.Context) {
	ctrl.record(c, "kora")
}

func (ctrl *WebhookController) Quidax(c *gin.Context) {
	ctrl.record(c, "quidax")
}

func (ctrl *WebhookController) record(c *gin.Context, provider string) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_webhook",
			Message: "Unable to read webhook payload",
		})
		return
	}

	if !ctrl.verifySignature(c, provider, body) {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "invalid_webhook_signature",
			Message: "Webhook signature verification failed",
		})
		return
	}

	envelope := parseWebhookEnvelope(body)
	_, err = ctrl.webhookService.Record(services.WebhookInput{
		Provider:  provider,
		EventID:   envelope.EventID,
		EventType: envelope.EventType,
		Reference: envelope.Reference,
		Payload:   json.RawMessage(body),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "webhook_record_failed",
			Message: "Unable to record webhook",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}

func (ctrl *WebhookController) verifySignature(c *gin.Context, provider string, body []byte) bool {
	switch provider {
	case "kora":
		return verifyKoraSignature(body, c.GetHeader("x-korapay-signature"), ctrl.koraWebhookSecret)
	case "quidax":
		return verifyQuidaxSignature(body, c.GetHeader("quidax-signature"), ctrl.quidaxWebhookSecret)
	default:
		return true
	}
}

type webhookEnvelope struct {
	EventID   string
	EventType string
	Reference string
}

func parseWebhookEnvelope(body []byte) webhookEnvelope {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return webhookEnvelope{}
	}

	envelope := webhookEnvelope{
		EventID:   stringField(payload, "id", "event_id", "reference"),
		EventType: stringField(payload, "event", "type"),
	}

	if data, ok := payload["data"].(map[string]interface{}); ok {
		if envelope.Reference == "" {
			envelope.Reference = stringField(data, "reference", "payment_reference", "transaction_reference")
		}
		if envelope.EventID == "" {
			envelope.EventID = stringField(data, "id", "reference", "transaction_reference")
		}
	}

	if envelope.Reference == "" {
		envelope.Reference = stringField(payload, "reference", "payment_reference", "transaction_reference")
	}

	return envelope
}

func stringField(payload map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func verifyKoraSignature(body []byte, signature, secret string) bool {
	if secret == "" {
		return true
	}
	if signature == "" {
		return false
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}

	dataRaw, ok := payload["data"]
	if !ok {
		return false
	}

	canonicalData, err := canonicalJSON(dataRaw)
	if err != nil {
		return false
	}

	expected := hmacSHA256Hex(canonicalData, secret)
	return hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected))
}

func verifyQuidaxSignature(body []byte, signatureHeader, secret string) bool {
	if secret == "" {
		return true
	}
	if signatureHeader == "" {
		return false
	}

	timestamp, signature := parseQuidaxSignature(signatureHeader)
	if timestamp == "" || signature == "" {
		return false
	}

	canonicalBody, err := canonicalJSON(body)
	if err != nil {
		return false
	}
	expected := hmacSHA256Hex([]byte(timestamp+"."+string(canonicalBody)), secret)
	if hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected)) {
		return true
	}

	// Some Quidax webhook examples sign only JSON.stringify(req.body).
	expected = hmacSHA256Hex(canonicalBody, secret)
	return hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected))
}

func canonicalJSON(raw []byte) ([]byte, error) {
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func hmacSHA256Hex(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func parseQuidaxSignature(header string) (string, string) {
	var timestamp string
	var signature string
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "t=") {
			timestamp = strings.TrimPrefix(part, "t=")
		}
		if strings.HasPrefix(part, "v1=") {
			signature = strings.TrimPrefix(part, "v1=")
		}
	}
	return timestamp, signature
}
