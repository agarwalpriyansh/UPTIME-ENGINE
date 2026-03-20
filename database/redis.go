package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"monitor-engine/models"

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

func SaveResult(result models.PingResult) error {
	// 1. Convert the Go struct into a JSON byte array
	jsonData, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	// 2. We use a background context for the database write
	ctx := context.Background()

	// 3. RPush (Right Push) adds this JSON string to the end of a list named "ping_buffer"
	err = RedisClient.RPush(ctx, "ping_buffer", jsonData).Err()
	if err != nil {
		return fmt.Errorf("failed to write to Redis: %v", err)
	}

	return nil
}