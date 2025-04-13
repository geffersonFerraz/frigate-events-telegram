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

// FlushAll limpa todos os dados do Redis
func (h *RedisHandler) FlushAll(ctx context.Context) error {
	return h.client.FlushAll(ctx).Err()
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

	// Usar SET com opção EX para definir a expiração em segundos (2 horas = 7200 segundos)
	if err := h.client.Set(ctx, key, "processed", 2*time.Hour).Err(); err != nil {
		return fmt.Errorf("erro ao marcar evento como processado: %w", err)
	}

	// Verificar se a expiração foi definida corretamente
	ttl, err := h.client.TTL(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("erro ao verificar TTL da chave: %w", err)
	}
	if ttl < 0 {
		return fmt.Errorf("erro: TTL não foi definido para a chave %s", key)
	}

	return nil
}

// Close fecha a conexão com o Redis
func (h *RedisHandler) Close() error {
	return h.client.Close()
}
