package cache

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisCache_StoreSent_Success(t *testing.T) {
	t.Parallel()

	// Start in-memory Redis
	mr := miniredis.RunT(t)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	ttl := 10 * time.Second
	cache := NewRedisCache(rdb, ttl)

	ctx := context.Background()
	internalID := int64(42)
	remoteID := "remote-123"
	sentAt := time.Date(2026, 2, 2, 18, 0, 0, 0, time.UTC)

	if err := cache.StoreSent(ctx, internalID, remoteID, sentAt); err != nil {
		t.Fatalf("StoreSent() error: %v", err)
	}

	key := "msg:42"

	if !mr.Exists(key) {
		t.Fatalf("expected key %q to exist", key)
	}

	ttlRemaining := mr.TTL(key)
	if ttlRemaining <= 0 {
		t.Fatalf("expected TTL to be set, got %v", ttlRemaining)
	}

	raw, err := mr.Get(key)
	if err != nil {
		t.Fatalf("failed to get key %q: %v", key, err)
	}

	var got sentValue
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("failed to unmarshal value: %v", err)
	}

	if got.RemoteMessageID != remoteID {
		t.Fatalf("expected RemoteMessageID %q, got %q", remoteID, got.RemoteMessageID)
	}
	if !got.SentAt.Equal(sentAt.UTC()) {
		t.Fatalf("expected SentAt %v, got %v", sentAt.UTC(), got.SentAt)
	}
}

func TestRedisCache_StoreSent_OverwritesExistingValue(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cache := NewRedisCache(rdb, time.Minute)
	ctx := context.Background()

	internalID := int64(1)

	// First write
	if err := cache.StoreSent(ctx, internalID, "first", time.Now()); err != nil {
		t.Fatalf("first StoreSent() error: %v", err)
	}

	// Second write should overwrite
	secondTime := time.Now().Add(time.Minute)
	if err := cache.StoreSent(ctx, internalID, "second", secondTime); err != nil {
		t.Fatalf("second StoreSent() error: %v", err)
	}

	raw, err := mr.Get("msg:1")
	if err != nil {
		t.Fatalf("failed to get key msg:1: %v", err)
	}

	var got sentValue
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("failed to unmarshal value: %v", err)
	}

	if got.RemoteMessageID != "second" {
		t.Fatalf("expected overwritten RemoteMessageID %q, got %q", "second", got.RemoteMessageID)
	}
}

func TestRedisCache_StoreSent_ContextCanceled(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cache := NewRedisCache(rdb, time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cache.StoreSent(ctx, 1, "x", time.Now())
	if err == nil {
		t.Fatalf("expected error due to canceled context, got nil")
	}
}
