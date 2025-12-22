package middleware

import (
	"net/http"
	"strings"
	"github.com/gin-gonic/gin"
	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/pkg/jwt"
)

func AuthMiddleware(jwtService *jwt.JWTService) gin.HandlerFunc{
	return func(c *gin.Context) {
		//Get authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error : "unauthorized",
				Message : "Authorization header missing",
			})
			c.Abort()
			return
		}

		//Check Bearer format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] !="Bearer"{
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error : "unauthorized",
				Message : "Invalid authorization header format",
			})
			c.Abort()
			return
		}
		token:=parts[1]
		
		//Validate token
		claims, err := jwtService.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error : "unauthorized",
				Message : "Invalid or expired token",
			})
			c.Abort()
			return
		}
		//Set user info in context
		c.Set("user_id", claims.UserID)
		c.Set("phone", claims.Phone)

		c.Next()
	}
}