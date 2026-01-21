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
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}
	jwt_secret := os.Getenv("JWT_SECRET")
	if jwt_secret == "" {
		log.Fatal("JWT SECRET is in env file")
	}
	return &Config{
		Port:                   getEnv("PORT", "8080"),
		DBHost:                 getEnv("DB_HOST", "localhost"),
		DBPort:                 getEnv("DB_PORT", "5432"),
		DBUser:                 getEnv("DB_USER", "payment_user"),
		DBPassword:             getEnv("DB_PASSWORD", "p1ssw4rd"),
		DBName:                 getEnv("DB_NAME", "payment_app_dev"),
		DBSSLMode:              getEnv("DB_SSLMODE", "disable"),
		RedisHost:              getEnv("REDIS_HOST", "localhost"),
		RedisPort:              getEnv("REDIS_PORT", "6379"),
		RedisPassword:          getEnv("REDIS_PASSWORD", ""),
		JWTSecret:              jwt_secret,
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
