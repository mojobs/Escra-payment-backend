package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"

	"github.com/mojobs/lara-payment-backend.git/internal/config"
	"github.com/mojobs/lara-payment-backend.git/internal/controllers"
	"github.com/mojobs/lara-payment-backend.git/internal/database"
	"github.com/mojobs/lara-payment-backend.git/internal/middleware"
	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/services"
	"github.com/mojobs/lara-payment-backend.git/pkg/jwt"
)

func main() {
	cfg := config.LoadConfig()

	if err := database.Connect(cfg); err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	db := database.GetDB()

	if err := db.AutoMigrate(&models.User{}, &models.Wallet{}, &models.Transaction{}, &models.LedgerEntry{}); err != nil {
		log.Fatal("Failed to migrate database: ", err)
	}

	//Initialize services and controllers
	jwtService, err := jwt.NewJWTService(cfg.JWTSecret)
	if err != nil {
		log.Fatalf("Failed to authenticate JWT service : %v", err)
	}
	userService := services.NewUserService(db)
	walletService := services.NewWalletService(db)
	authService := services.NewAuthService(userService, walletService, jwtService)
	transactionService := services.NewTransactionService(db, userService, walletService)

	//Initialize Controllers
	authController := controllers.NewAuthController(authService)
	userController := controllers.NewUserController(userService)
	walletController := controllers.NewWalletController(walletService)
	transactionController := controllers.NewTransactionController(transactionService)

	// Setup Gin router
	if cfg.Environment == "development" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	//Health Check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "Payment API is running",
		})
	})

	v1 := router.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authController.Register)
			auth.POST("/login", authController.Login)
		}

		// Protected routes
		protected := v1.Group("")
		{
			protected.Use(middleware.AuthMiddleware(jwtService))
			// User routes
			users := protected.Group("/users")
			{
				users.GET("/profile", userController.GetProfile)
			}

			// Wallet routes
			wallets := protected.Group("/wallets")
			{
				wallets.GET("/balance", walletController.GetBalance)
				wallets.GET("/", walletController.GetWallet)
			}

			transactioons := protected.Group("/transactions")
			{
				transactioons.POST("/transfer", transactionController.Transfer)
				transactioons.GET("/history", transactionController.GetHistory)
				transactioons.GET("/:reference", transactionController.GetByReference)
			}	
		}
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Server starting on port %s", addr)

	if err := router.Run(addr); err != nil {
		log.Fatal("Failed to start server: ", err)
	}
}
