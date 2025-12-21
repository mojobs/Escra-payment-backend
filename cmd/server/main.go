package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"

	"github.com/mojobs/lara-payment-backend.git/internal/config"
	"github.com/mojobs/lara-payment-backend.git/internal/database"
	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/controllers"
	"github.com/mojobs/lara-payment-backend.git/internal/services"
	"github.com/mojobs/lara-payment-backend.git/pkg/jwt"
)

func main() {
	cfg := config.LoadConfig()

	if err := database.Connect(cfg); err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	db:= database.GetDB()

	if err := db.AutoMigrate(&models.User{}); err != nil {
		log.Fatal("Failed to migrate database: ", err)
	}

	jwtService, err := jwt.NewJWTService(cfg.JWTSecret)
	if err != nil {
		log.Fatalf("Failed to authenticate JWT service : %v", err)
	}
	userService := services.NewUserService(db)
	authService := services.NewAuthService(userService, jwtService)

	authController := controllers.NewAuthController(authService)

	if cfg.Environment == "development" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "Payment API is running",
		})
	})

	authRoutes := router.Group("/api/v1/auth")
	{
		authRoutes.POST("/register",authController.Register)
		authRoutes.POST("/login", authController.Login)
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Server starting on port %s", addr)

	if err := router.Run(addr); err != nil {
		log.Fatal("Failed to start server: ", err)
	}
}
