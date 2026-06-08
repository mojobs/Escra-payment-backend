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
	"github.com/mojobs/lara-payment-backend.git/internal/providers/kora"
	"github.com/mojobs/lara-payment-backend.git/internal/providers/quidax"
	"github.com/mojobs/lara-payment-backend.git/internal/services"
	"github.com/mojobs/lara-payment-backend.git/pkg/jwt"
)

func main() {
	cfg := config.LoadConfig()

	if err := database.Connect(cfg); err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	if err := database.ConnectRedis(cfg); err != nil {
		log.Fatal("Failed to connect to Redis: ", err)
	}

	db := database.GetDB()

	log.Println("Running Migrations")
	if err := db.AutoMigrate(&models.User{}, &models.Wallet{}, &models.Transaction{}, &models.LedgerEntry{}, &models.IdempotencyKey{}, &models.TransactionLimit{}, &models.TransactionUsage{}, &models.ProviderTransaction{}, &models.WebhookEvent{}, &models.KoraVirtualAccount{}, &models.AuditLog{}, &models.EscrowOrder{}, &models.EscrowEvent{}, &models.EscrowDispute{}); err != nil {
		log.Fatal("Failed to migrate database: ", err)
	}
	log.Println("Migrations completed successfully")

	//Initialize services and controllers
	jwtService, err := jwt.NewJWTService(cfg.JWTSecret)
	if err != nil {
		log.Fatalf("Failed to authenticate JWT service : %v", err)
	}
	userService := services.NewUserService(db)
	walletService := services.NewWalletService(db)
	limitService := services.NewTransactionLimitService(db)
	authService := services.NewAuthService(userService, walletService, limitService, jwtService)
	transactionService := services.NewTransactionService(db, userService, walletService, limitService)
	escrowService := services.NewEscrowService(db, userService, walletService, limitService)
	idempotencyService := services.NewIdempotencyService(db)
	koraWebhookSecret := cfg.KoraWebhookSecret
	if koraWebhookSecret == "" {
		koraWebhookSecret = cfg.KoraSecretKey
	}
	providerService := services.NewProviderService(
		db,
		userService,
		walletService,
		limitService,
		kora.NewClient(cfg.KoraBaseURL, cfg.KoraPublicKey, cfg.KoraSecretKey),
		quidax.NewClient(cfg.QuidaxBaseURL, cfg.QuidaxSecretKey),
	)
	webhookService := services.NewWebhookService(db, escrowService)

	//Initialize Controllers
	authController := controllers.NewAuthController(authService)
	userController := controllers.NewUserController(userService)
	walletController := controllers.NewWalletController(walletService, providerService, cfg.PublicBaseURL)
	transactionController := controllers.NewTransactionController(transactionService)
	escrowController := controllers.NewEscrowController(escrowService, providerService, cfg.PublicBaseURL)
	limitController := controllers.NewLimitController(limitService)
	webhookController := controllers.NewWebhookController(webhookService, koraWebhookSecret, cfg.QuidaxWebhookSecret)
	providerController := controllers.NewProviderController(providerService)

	// Setup Gin router
	if cfg.Environment != "development" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	//Apply general rate limiting to all routes
	router.Use(middleware.RateLimiterMiddleware())

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
		auth.Use(middleware.StrictRateLimiterMiddleware())
		{
			auth.POST("/register", authController.Register)
			auth.POST("/login", authController.Login)
		}

		webhooks := v1.Group("/webhooks")
		{
			webhooks.POST("/kora", webhookController.Kora)
			webhooks.POST("/quidax", webhookController.Quidax)
		}

		public := v1.Group("/public")
		{
			public.GET("/escrows/orders/:reference", escrowController.GetPublicOrder)
		}

		admin := v1.Group("/admin")
		admin.Use(middleware.AdminAPIKeyMiddleware(cfg.AdminAPIKey))
		{
			admin.POST("/escrows/orders/:reference/resolve-dispute", escrowController.ResolveDispute)
			admin.POST("/providers/kora/refunds", providerController.InitiateKoraRefund)
		}

		// Protected routes
		protected := v1.Group("")
		{
			protected.Use(middleware.AuthMiddleware(jwtService))
			protected.Use(middleware.UserRateLimiterMiddleware())
			protected.Use(middleware.IdempotencyMiddleware(idempotencyService))
			// User routes
			users := protected.Group("/users")
			{
				users.GET("/profile", userController.GetProfile)
				users.PUT("/profile/business", userController.UpdateMerchantDetails)
			}

			// Wallet routes
			wallets := protected.Group("/wallets")
			{
				wallets.GET("/balance", walletController.GetBalance)
				wallets.POST("/checkout/kora", middleware.StrictRateLimiterMiddleware(), walletController.InitiateKoraCheckout)
				wallets.GET("/", walletController.GetWallet)
			}

			transactions := protected.Group("/transactions")
			{
				transactions.POST("/transfer", middleware.StrictRateLimiterMiddleware(), transactionController.Transfer)
				transactions.GET("/history", transactionController.GetHistory)
				transactions.GET("/:reference", transactionController.GetByReference)
			}

			escrows := protected.Group("/escrows")
			{
				escrows.POST("/orders", escrowController.CreateOrder)
				escrows.GET("/orders", escrowController.ListOrders)
				escrows.GET("/orders/:reference", escrowController.GetOrder)
				escrows.POST("/orders/:reference/checkout/kora", middleware.StrictRateLimiterMiddleware(), escrowController.InitiateKoraCheckout)
				escrows.POST("/orders/:reference/fund", middleware.StrictRateLimiterMiddleware(), escrowController.FundOrder)
				escrows.POST("/orders/:reference/ship", escrowController.MarkShipped)
				escrows.POST("/orders/:reference/mark-delivered", escrowController.MarkDelivered)
				escrows.POST("/orders/:reference/confirm-delivery", escrowController.ConfirmDelivery)
				escrows.POST("/orders/:reference/release", escrowController.ReleaseIfEligible)
				escrows.POST("/orders/:reference/dispute", escrowController.OpenDispute)
				escrows.POST("/orders/:reference/cancel", escrowController.CancelOrder)
				escrows.POST("/orders/:reference/refund", escrowController.RefundOrder)
			}

			limits := protected.Group("/limits")
			{
				limits.GET("/", limitController.GetLimits)
			}

			providers := protected.Group("/providers")
			{
				koraRoutes := providers.Group("/kora")
				{
					koraRoutes.POST("/kyc/verify", middleware.StrictRateLimiterMiddleware(), providerController.VerifyKoraIdentity)
					koraRoutes.GET("/banks", providerController.ListKoraBanks)
					koraRoutes.GET("/banks/resolve", providerController.ResolveKoraBankAccount)
					koraRoutes.GET("/balances", providerController.GetKoraBalances)
					koraRoutes.POST("/virtual-accounts", middleware.StrictRateLimiterMiddleware(), providerController.CreateKoraVirtualAccount)
					koraRoutes.GET("/virtual-accounts", providerController.ListKoraVirtualAccounts)
					koraRoutes.POST("/payouts/bank", middleware.StrictRateLimiterMiddleware(), providerController.InitiateKoraBankPayout)
				}

				quidaxRoutes := providers.Group("/quidax")
				{
					quidaxRoutes.POST("/withdrawals", middleware.StrictRateLimiterMiddleware(), providerController.InitiateQuidaxWithdrawal)
				}
			}
		}
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Server starting on port %s", addr)

	if err := router.Run(addr); err != nil {
		log.Fatal("Failed to start server: ", err)
	}
}
