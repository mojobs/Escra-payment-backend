package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                   string
	DBHost                 string
	DBPort                 string
	DBUser                 string
	DBPassword             string
	DBName                 string
	DBSSLMode              string
	JWTSecret              string
	JWTExpiration          string
	RefreshTokenExpiration string
	Environment            string
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}
	return &Config{
		Port:                   getEnv("PORT", "8080"),
		DBHost:                 getEnv("DB_HOST", "localhost"),
		DBPort:                 getEnv("DB_PORT", "5432"),
		DBUser:                 getEnv("DB_USER", "payment_user"),
		DBPassword:             getEnv("DB_PASSWORD", "p1ssw4rd"),
		DBName:                 getEnv("DB_NAME", "payment_app_dev"),
		DBSSLMode:              getEnv("DB_SSLMODE", "disable"),
		JWTSecret:              getEnv("JWT_SECRET", "secret"),
		JWTExpiration:          getEnv("JWT_EXPIRATION", "15m"),
		RefreshTokenExpiration: getEnv("REFRESH_TOKEN_EXPIRATION", "168h"),
		Environment:            getEnv("ENVIRONMENT", "development"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
