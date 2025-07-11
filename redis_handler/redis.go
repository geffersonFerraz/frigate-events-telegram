package redis_handler

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisHandler manages the Redis connection
type RedisHandler struct {
	client *redis.Client
}

// NewRedisHandler creates a new instance of RedisHandler
func NewRedisHandler(addr, password string, db int) (*RedisHandler, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Test the connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("error connecting to Redis: %w", err)
	}

	return &RedisHandler{
		client: client,
	}, nil
}

// FlushAll clears all data from Redis
func (h *RedisHandler) FlushAll(ctx context.Context) error {
	return h.client.FlushAll(ctx).Err()
}

// IsEventProcessed checks if an event has already been processed
func (h *RedisHandler) IsEventProcessed(ctx context.Context, eventID string, eventType string) (bool, error) {
	key := fmt.Sprintf("frigate:event:%s:%s", eventType, eventID)
	exists, err := h.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("error checking event in Redis: %w", err)
	}
	return exists > 0, nil
}

// MarkEventAsProcessed marks an event as processed
func (h *RedisHandler) MarkEventAsProcessed(ctx context.Context, eventID string, eventType string) error {
	key := fmt.Sprintf("frigate:event:%s:%s", eventType, eventID)

	// Use SET with EX option to set expiration in seconds (2 hours = 7200 seconds)
	if err := h.client.Set(ctx, key, "processed", 2*time.Hour).Err(); err != nil {
		return fmt.Errorf("error marking event as processed: %w", err)
	}

	// Check if expiration was set correctly
	ttl, err := h.client.TTL(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("error checking key TTL: %w", err)
	}
	if ttl < 0 {
		return fmt.Errorf("error: TTL was not set for key %s", key)
	}

	return nil
}

// Close closes the Redis connection
func (h *RedisHandler) Close() error {
	return h.client.Close()
}
