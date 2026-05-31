package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client            *redis.Client
	metadataKeyPrefix string
	aclKeyPrefix      string
	ttl               time.Duration
}

func NewRedisCache(addr, metadataKeyPrefix, aclKeyPrefix string, ttl time.Duration) *RedisCache {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &RedisCache{
		client: redis.NewClient(&redis.Options{Addr: addr}),
		metadataKeyPrefix: metadataKeyPrefix,
		aclKeyPrefix:      aclKeyPrefix,
		ttl:               ttl,
	}
}

func (c *RedisCache) Close() error { return c.client.Close() }

func (c *RedisCache) metadataKey(assetID string) string {
	if strings.Contains(c.metadataKeyPrefix, "{assetId}") {
		return strings.ReplaceAll(c.metadataKeyPrefix, "{assetId}", assetID)
	}
	return fmt.Sprintf("%s:%s", c.metadataKeyPrefix, assetID)
}

func (c *RedisCache) aclKey(assetID string) string {
	if strings.Contains(c.aclKeyPrefix, "{assetId}") {
		return strings.ReplaceAll(c.aclKeyPrefix, "{assetId}", assetID)
	}
	return fmt.Sprintf("%s:%s", c.aclKeyPrefix, assetID)
}

func (c *RedisCache) GetMetadata(ctx context.Context, assetID string) ([]byte, bool, error) {
	value, err := c.client.Get(ctx, c.metadataKey(assetID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, err
	}
	return value, true, nil
}

func (c *RedisCache) SetMetadata(ctx context.Context, assetID string, metadata []byte) error {
	return c.client.Set(ctx, c.metadataKey(assetID), metadata, c.ttl).Err()
}

func (c *RedisCache) DeleteMetadata(ctx context.Context, assetID string) error {
	return c.client.Del(ctx, c.metadataKey(assetID)).Err()
}

func (c *RedisCache) GetACL(ctx context.Context, assetID string) ([]string, bool, error) {
	value, err := c.client.Get(ctx, c.aclKey(assetID)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, err
	}
	var acl []string
	if err := json.Unmarshal([]byte(value), &acl); err != nil {
		return nil, false, err
	}
	return acl, true, nil
}

func (c *RedisCache) SetACL(ctx context.Context, assetID string, acl []string) error {
	body, err := json.Marshal(acl)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, c.aclKey(assetID), body, c.ttl).Err()
}
