package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
)

func AdminAPIKeyMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" {
			c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{
				Error:   "admin_unavailable",
				Message: "admin api key is not configured",
			})
			c.Abort()
			return
		}

		if c.GetHeader("X-Admin-Key") != secret {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error:   "unauthorized",
				Message: "invalid admin api key",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
