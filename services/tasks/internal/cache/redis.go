package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

// Task es la estructura que vamos a cachear (debe coincidir con tu modelo)
type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	DueDate     string `json:"due_date"`
	Status      string `json:"status"`
	UserID      string `json:"user_id"`
}

// RedisCache maneja las operaciones de caché
type RedisCache struct {
	client  *redis.Client
	baseTTL time.Duration
	jitter  time.Duration
}

// NewRedisCache crea una nueva instancia del caché
func NewRedisCache(addr, password string, db int, baseTTLSeconds, jitterSeconds int) *RedisCache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
		// Timeouts importantes para no bloquear
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
		DialTimeout:  1 * time.Second,
	})

	return &RedisCache{
		client:  rdb,
		baseTTL: time.Duration(baseTTLSeconds) * time.Second,
		jitter:  time.Duration(jitterSeconds) * time.Second,
	}
}

// GetTask obtiene una tarea del caché
func (c *RedisCache) GetTask(ctx context.Context, id string) (*Task, error) {
	key := fmt.Sprintf("tasks:task:%s", id)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		log.Printf("[WARN] Error deserializando tarea del caché: %v", err)
		return nil, err
	}

	log.Printf("[CACHE HIT] Tarea %s encontrada en Redis", id)
	return &task, nil
}

// SetTask guarda una tarea en caché con TTL + jitter
func (c *RedisCache) SetTask(ctx context.Context, task *Task) error {
	key := fmt.Sprintf("tasks:task:%s", task.ID)

	data, err := json.Marshal(task)
	if err != nil {
		return err
	}

	// Aplicar jitter: TTL base + valor aleatorio entre 0 y jitter
	jitterValue := time.Duration(rand.Int63n(int64(c.jitter)))
	ttl := c.baseTTL + jitterValue

	err = c.client.Set(ctx, key, data, ttl).Err()
	if err == nil {
		log.Printf("[CACHE SET] Tarea %s guardada por %v (base: %v, jitter: +%v)",
			task.ID, ttl, c.baseTTL, jitterValue)
	}

	return err
}

// DeleteTask elimina una tarea del caché (invalidación)
func (c *RedisCache) DeleteTask(ctx context.Context, id string) error {
	key := fmt.Sprintf("tasks:task:%s", id)
	err := c.client.Del(ctx, key).Err()
	if err == nil {
		log.Printf("[CACHE DEL] Caché invalidado para tarea %s", id)
	}
	return err
}

// Ping verifica la conexión con Redis
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close cierra la conexión
func (c *RedisCache) Close() error {
	return c.client.Close()
}