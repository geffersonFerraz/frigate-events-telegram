package redis_handler

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisHandler gerencia a conexão com o Redis
type RedisHandler struct {
	client *redis.Client
}

// NewRedisHandler cria uma nova instância do RedisHandler
func NewRedisHandler(addr, password string, db int) (*RedisHandler, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Testar a conexão
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("erro ao conectar ao Redis: %w", err)
	}

	return &RedisHandler{
		client: client,
	}, nil
}

// IsEventProcessed verifica se um evento já foi processado
func (h *RedisHandler) IsEventProcessed(ctx context.Context, eventID string, eventType string) (bool, error) {
	key := fmt.Sprintf("frigate:event:%s:%s", eventType, eventID)
	exists, err := h.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("erro ao verificar evento no Redis: %w", err)
	}
	return exists > 0, nil
}

// MarkEventAsProcessed marca um evento como processado
func (h *RedisHandler) MarkEventAsProcessed(ctx context.Context, eventID string, eventType string) error {
	key := fmt.Sprintf("frigate:event:%s:%s", eventType, eventID)
	// Armazenar o evento por 24 horas
	if err := h.client.Set(ctx, key, "processed", 24*time.Hour).Err(); err != nil {
		return fmt.Errorf("erro ao marcar evento como processado: %w", err)
	}
	return nil
}

// Close fecha a conexão com o Redis
func (h *RedisHandler) Close() error {
	return h.client.Close()
}
