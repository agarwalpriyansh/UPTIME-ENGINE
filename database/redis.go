package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"monitor-engine/models"
)

// PingBufferKey is the Redis list that buffers ping JSON between workers and the flusher.
// Must match the key used in worker/flusher.go.
const PingBufferKey = "ping_buffer"

// RedisClient is the shared connection pool (initialized by InitRedis).
var RedisClient *redis.Client

// InitRedis connects to Redis using REDIS_ADDR and optional REDIS_PASSWORD.
func InitRedis() error {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	password := os.Getenv("REDIS_PASSWORD")

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: password,
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := RedisClient.Ping(ctx).Result(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Println("[DATABASES] Successfully connected to Redis!")
	return nil
}

// SaveResult appends one ping result JSON to the Redis buffer list.
func SaveResult(result models.PingResult) error {
	jsonData, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := RedisClient.RPush(ctx, PingBufferKey, jsonData).Err(); err != nil {
		return fmt.Errorf("failed to write to Redis: %w", err)
	}
	return nil
}
