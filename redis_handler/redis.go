package redis_handler

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisHandler struct {
	client *redis.Client
}

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

func (h *RedisHandler) FlushAll(ctx context.Context) error {
	return h.client.FlushAll(ctx).Err()
}

func (h *RedisHandler) IsEventProcessed(ctx context.Context, eventID string, eventType string) (bool, error) {
	key := fmt.Sprintf("frigate:event:%s:%s", eventType, eventID)
	exists, err := h.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("error checking event in Redis: %w", err)
	}
	return exists > 0, nil
}

func (h *RedisHandler) IsEventProcessing(ctx context.Context, eventID string, eventType string) (bool, error) {
	key := fmt.Sprintf("frigate:event:%s:%s:processing", eventType, eventID)
	exists, err := h.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("error checking event in Redis: %w", err)
	}
	return exists > 0, nil
}

func (h *RedisHandler) MarkEventAsProcessing(ctx context.Context, eventID string, eventType string) error {
	key := fmt.Sprintf("frigate:event:%s:%s:processing", eventType, eventID)

	if err := h.client.Set(ctx, key, "processing", 30*time.Second).Err(); err != nil {
		return fmt.Errorf("error marking event as processing: %w", err)
	}

	ttl, err := h.client.TTL(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("error checking key TTL: %w", err)
	}
	if ttl < 0 {
		return fmt.Errorf("error: TTL was not set for key %s", key)
	}

	return nil
}

func (h *RedisHandler) MarkEventAsProcessed(ctx context.Context, eventID string, eventType string) error {
	key := fmt.Sprintf("frigate:event:%s:%s", eventType, eventID)

	if err := h.client.Set(ctx, key, "processed", 2*time.Hour).Err(); err != nil {
		return fmt.Errorf("error marking event as processed: %w", err)
	}

	ttl, err := h.client.TTL(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("error checking key TTL: %w", err)
	}
	if ttl < 0 {
		return fmt.Errorf("error: TTL was not set for key %s", key)
	}

	return nil
}

func (h *RedisHandler) Close() error {
	return h.client.Close()
}
