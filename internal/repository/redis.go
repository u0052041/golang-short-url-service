package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jack/golang-short-url-service/internal/config"
	"github.com/jack/golang-short-url-service/internal/model"
	"github.com/redis/go-redis/v9"
)

const (
	urlCachePrefix   = "url:"
	clickCountPrefix = "clicks:"
	urlCacheTTL      = 1 * time.Hour
)

type RedisRepository struct {
	client *redis.Client
}

func NewRedisRepository(cfg *config.RedisConfig) (*RedisRepository, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return &RedisRepository{client: client}, nil
}

func (r *RedisRepository) Close() error {
	return r.client.Close()
}

func (r *RedisRepository) Client() *redis.Client {
	return r.client
}

func (r *RedisRepository) GetURL(ctx context.Context, shortCode string) (*model.URL, error) {
	key := urlCachePrefix + shortCode

	// 用 GETEX：讀取同時刷新 TTL（Redis 6.2+），避免熱門 key 失效造成抖動。
	data, err := r.client.GetEx(ctx, key, urlCacheTTL).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, fmt.Errorf("failed to get url from cache: %w", err)
	}

	var url model.URL
	if err := json.Unmarshal(data, &url); err != nil {
		return nil, fmt.Errorf("failed to unmarshal url: %w", err)
	}

	return &url, nil
}

func (r *RedisRepository) SetURL(ctx context.Context, url *model.URL) error {
	key := urlCachePrefix + url.ShortCode

	data, err := json.Marshal(url)
	if err != nil {
		return fmt.Errorf("failed to marshal url: %w", err)
	}

	ttl := urlCacheTTL
	if url.ExpiresAt != nil {
		remaining := time.Until(*url.ExpiresAt)
		if remaining < ttl {
			ttl = remaining
		}
	}

	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set url in cache: %w", err)
	}

	return nil
}

func (r *RedisRepository) DeleteURL(ctx context.Context, shortCode string) error {
	key := urlCachePrefix + shortCode

	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete url from cache: %w", err)
	}

	return nil
}

func (r *RedisRepository) IncrementClickCount(ctx context.Context, shortCode string) error {
	key := clickCountPrefix + shortCode

	if err := r.client.Incr(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to increment click count: %w", err)
	}

	return nil
}

func (r *RedisRepository) IncrementClickCountBy(ctx context.Context, shortCode string, delta int64) error {
	key := clickCountPrefix + shortCode

	if err := r.client.IncrBy(ctx, key, delta).Err(); err != nil {
		return fmt.Errorf("failed to increment click count by %d: %w", delta, err)
	}

	return nil
}

func (r *RedisRepository) GetClickCount(ctx context.Context, shortCode string) (int64, error) {
	key := clickCountPrefix + shortCode

	count, err := r.client.Get(ctx, key).Int64()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get click count: %w", err)
	}

	return count, nil
}

func (r *RedisRepository) GetAndResetClickCount(ctx context.Context, shortCode string) (int64, error) {
	key := clickCountPrefix + shortCode

	// 用 GETDEL：同步用「取值+刪除」原子操作，避免同步期間遺漏/重複計數（Redis 6.2+）。
	count, err := r.client.GetDel(ctx, key).Int64()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get and reset click count: %w", err)
	}

	return count, nil
}

func (r *RedisRepository) GetAllClickCountKeys(ctx context.Context) ([]string, error) {
	pattern := clickCountPrefix + "*"

	var keys []string
	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan click count keys: %w", err)
	}

	return keys, nil
}

func ExtractShortCodeFromKey(key string) string {
	return key[len(clickCountPrefix):]
}

func (r *RedisRepository) Health(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}
