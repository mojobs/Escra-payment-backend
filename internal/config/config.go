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
	RedisHost              string
	RedisPort              string
	RedisPassword          string
	JWTSecret              string
	JWTExpiration          string
	RefreshTokenExpiration string
	Environment            string
	PublicBaseURL          string
	AdminAPIKey            string
	KoraBaseURL            string
	KoraPublicKey          string
	KoraSecretKey          string
	KoraWebhookSecret      string
	QuidaxBaseURL          string
	QuidaxSecretKey        string
	QuidaxWebhookSecret    string
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found; using process environment")
	}
	jwt_secret := os.Getenv("JWT_SECRET")
	if jwt_secret == "" {
		log.Fatal("JWT_SECRET is required")
	}
	return &Config{
		Port:                   getEnv("PORT", "8080"),
		DBHost:                 getEnv("DB_HOST", "localhost"),
		DBPort:                 getEnv("DB_PORT", "5432"),
		DBUser:                 getEnv("DB_USER", "payment_user"),
		DBPassword:             getEnv("DB_PASSWORD", ""),
		DBName:                 getEnv("DB_NAME", "payment_app_dev"),
		DBSSLMode:              getEnv("DB_SSLMODE", "disable"),
		RedisHost:              getEnv("REDIS_HOST", "localhost"),
		RedisPort:              getEnv("REDIS_PORT", "6379"),
		RedisPassword:          getEnv("REDIS_PASSWORD", ""),
		JWTSecret:              jwt_secret,
		JWTExpiration:          getEnv("JWT_EXPIRATION", "15m"),
		RefreshTokenExpiration: getEnv("REFRESH_TOKEN_EXPIRATION", "168h"),
		Environment:            getEnv("ENVIRONMENT", "development"),
		PublicBaseURL:          getEnv("PUBLIC_BASE_URL", ""),
		AdminAPIKey:            getEnv("ADMIN_API_KEY", ""),
		KoraBaseURL:            getEnv("KORA_BASE_URL", "https://api.korapay.com"),
		KoraPublicKey:          getEnv("KORA_PUBLIC_KEY", ""),
		KoraSecretKey:          getEnv("KORA_SECRET_KEY", ""),
		KoraWebhookSecret:      getEnv("KORA_WEBHOOK_SECRET", ""),
		QuidaxBaseURL:          getEnv("QUIDAX_BASE_URL", "https://openapi.quidax.io/exchange-open-api/v1"),
		QuidaxSecretKey:        getEnv("QUIDAX_SECRET_KEY", ""),
		QuidaxWebhookSecret:    getEnv("QUIDAX_WEBHOOK_SECRET", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
