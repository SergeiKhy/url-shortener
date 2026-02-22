package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SergeiKhy/url-shortener/internal/models"
)

type CacheRepository interface {
	Get(ctx context.Context, key string) (*models.Link, error)
	Set(ctx context.Context, key string, link *models.Link, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

type cacheRepository struct {
	redis *RedisDB
}

func NewCacheRepository(redis *RedisDB) CacheRepository {
	return &cacheRepository{redis: redis}
}

func (r *cacheRepository) Get(ctx context.Context, key string) (*models.Link, error) {
	data, err := r.redis.Client.Get(ctx, r.key(key)).Bytes()
	if err != nil {
		return nil, err
	}

	var link models.Link
	if err := json.Unmarshal(data, &link); err != nil {
		return nil, fmt.Errorf("failed to unmarshal link: %w", err)
	}

	return &link, nil
}

func (r *cacheRepository) Set(ctx context.Context, key string, link *models.Link, ttl time.Duration) error {
	data, err := json.Marshal(link)
	if err != nil {
		return fmt.Errorf("failed to marshal link: %w", err)
	}

	return r.redis.Client.Set(ctx, r.key(key), data, ttl).Err()
}

func (r *cacheRepository) Delete(ctx context.Context, key string) error {
	return r.redis.Client.Del(ctx, r.key(key)).Err()
}

func (r *cacheRepository) key(key string) string {
	return "link:" + key
}
