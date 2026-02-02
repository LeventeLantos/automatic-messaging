package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewRedisCache(rdb *redis.Client, ttl time.Duration) *RedisCache {
	return &RedisCache{rdb: rdb, ttl: ttl}
}

type sentValue struct {
	RemoteMessageID string    `json:"remoteMessageId"`
	SentAt          time.Time `json:"sentAt"`
}

func (c *RedisCache) StoreSent(ctx context.Context, internalID int64, remoteMessageID string, sentAt time.Time) error {
	key := fmt.Sprintf("msg:%d", internalID)
	val := sentValue{
		RemoteMessageID: remoteMessageID,
		SentAt:          sentAt.UTC(),
	}

	b, err := json.Marshal(val)
	if err != nil {
		return err
	}

	return c.rdb.Set(ctx, key, b, c.ttl).Err()
}
