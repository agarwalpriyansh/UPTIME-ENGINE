package database

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// We make this variable accessible to other packages
var RedisClient *redis.Client

// InitRedis connects to our Dockerized Redis instance
func InitRedis() error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // The port we exposed in docker-compose
		Password: "",               // No password for local development
		DB:       0,                // Default DB
	})

	// Create a short timeout context just for the ping
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Ping the database to check if the connection is alive
	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %v", err)
	}

	fmt.Println("[DATABASES] Successfully connected to Redis!")
	return nil
}