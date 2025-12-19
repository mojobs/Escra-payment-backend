package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"

	"github.com/mojobs/lara-payment-backend.git/internal/config"
	"github.com/mojobs/lara-payment-backend.git/internal/database"
)

func main() {
	cfg := config.LoadConfig()

	if err := database.Connect(cfg); err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

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

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Server starting on port %s", addr)

	if err := router.Run(addr); err != nil {
		log.Fatal("Failed to start server: ", err)
	}
}
