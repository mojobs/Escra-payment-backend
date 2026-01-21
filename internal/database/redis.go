package database

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
	"github.com/mojobs/lara-payment-backend.git/internal/config"
)

var RedisClient *redis.Client
var ctx = context.Background()

func ConnectRedis(cfg *config.Config) error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB : 0,
	})

	if err := RedisClient.Ping(ctx).Err(); err != nil{
		return fmt.Errorf("Failed to connect to Redis: %w", err)
	}

	log.Println("Redis connected successfully")
	return nil
}

func GetRedis() *redis.Client {
	return RedisClient
}

func GetContext() context.Context{
	return ctx
}