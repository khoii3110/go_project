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
	client    *redis.Client
	keyPrefix string
	ttl       time.Duration
}

func NewRedisCache(addr, keyPrefix string, ttl time.Duration) *RedisCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &RedisCache{
		client: redis.NewClient(&redis.Options{Addr: addr}),
		keyPrefix: keyPrefix,
		ttl: ttl,
	}
}

func (c *RedisCache) Close() error {
	return c.client.Close()
}

func (c *RedisCache) MembersKey(teamID string) string {
	if strings.Contains(c.keyPrefix, "{teamId}") {
		return strings.ReplaceAll(c.keyPrefix, "{teamId}", teamID)
	}
	return fmt.Sprintf("%s:%s", c.keyPrefix, teamID)
}

func (c *RedisCache) GetMembers(ctx context.Context, teamID string) ([]string, bool, error) {
	value, err := c.client.Get(ctx, c.MembersKey(teamID)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, err
	}

	var members []string
	if err := json.Unmarshal([]byte(value), &members); err != nil {
		return nil, false, err
	}

	return members, true, nil
}

func (c *RedisCache) SetMembers(ctx context.Context, teamID string, members []string) error {
	body, err := json.Marshal(members)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, c.MembersKey(teamID), body, c.ttl).Err()
}

func (c *RedisCache) DeleteMembers(ctx context.Context, teamID string) error {
	return c.client.Del(ctx, c.MembersKey(teamID)).Err()
}
