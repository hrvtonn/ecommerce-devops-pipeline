package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/config"
)

// Default TTLs per resource type.
var defaultTTL = map[string]time.Duration{
	"product":         5 * time.Minute,
	"products_list":   2 * time.Minute,
	"category":        10 * time.Minute,
	"categories_list": 10 * time.Minute,
	"order":           1 * time.Minute,
	"customer":        5 * time.Minute,
}

// TTL returns the default TTL for a resource type.
func TTL(kind string) time.Duration {
	if ttl, ok := defaultTTL[kind]; ok {
		return ttl
	}
	return 5 * time.Minute
}

// NewRedisClient creates the Redis client. It returns nil under tests, which
// disables caching.
func NewRedisClient(cfg config.Config) *redis.Client {
	if cfg.IsTest() {
		return nil
	}
	return redis.NewClient(&redis.Options{
		Addr:            cfg.Redis.Addr(),
		Password:        cfg.Redis.Password,
		DB:              cfg.Redis.DB,
		MaxRetries:      3,
		MinRetryBackoff: 200 * time.Millisecond,
		MaxRetryBackoff: 5 * time.Second,
		DialTimeout:     2 * time.Second,
	})
}

// Cache is a JSON cache on top of Redis. Every failure is logged and treated
// as a cache miss, so the application keeps working when Redis is down.
type Cache struct {
	client *redis.Client
}

// New creates a Cache. A nil client yields a no-op cache.
func New(client *redis.Client) *Cache { return &Cache{client: client} }

// Get decodes the cached value for key into dest and reports whether it was found.
func (c *Cache) Get(ctx context.Context, key string, dest any) bool {
	if c.client == nil {
		return false
	}

	data, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		slog.Debug("Cache MISS: " + key)
		return false
	}
	if err == nil {
		err = json.Unmarshal(data, dest)
	}
	if err != nil {
		slog.Warn("Cache get failed, falling back to database", "key", key, "error", err)
		return false
	}

	slog.Debug("Cache HIT: " + key)
	return true
}

// Set stores value as JSON. A zero ttl stores it without expiration.
func (c *Cache) Set(ctx context.Context, key string, value any, ttl time.Duration) {
	if c.client == nil {
		return
	}

	data, err := json.Marshal(value)
	if err == nil {
		err = c.client.Set(ctx, key, data, ttl).Err()
	}
	if err != nil {
		slog.Warn("Cache set failed", "key", key, "error", err)
		return
	}

	slog.Debug("Cache SET: "+key, "ttl", ttl)
}

func (c *Cache) Del(ctx context.Context, key string) {
	if c.client == nil {
		return
	}

	if err := c.client.Del(ctx, key).Err(); err != nil {
		slog.Warn("Cache del failed", "key", key, "error", err)
		return
	}

	slog.Debug("Cache DEL: " + key)
}

// InvalidatePattern deletes every key matching pattern (e.g. "products:*").
// It uses SCAN to avoid blocking Redis.
func (c *Cache) InvalidatePattern(ctx context.Context, pattern string) {
	if c.client == nil {
		return
	}

	var cursor uint64
	deleted := 0
	for {
		keys, next, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			slog.Warn("Cache invalidatePattern failed", "pattern", pattern, "error", err)
			return
		}

		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				slog.Warn("Cache invalidatePattern failed", "pattern", pattern, "error", err)
				return
			}
			deleted += len(keys)
		}

		cursor = next
		if cursor == 0 {
			break
		}
	}

	if deleted > 0 {
		slog.Debug("Cache INVALIDATE: "+pattern, "deleted", deleted)
	}
}
