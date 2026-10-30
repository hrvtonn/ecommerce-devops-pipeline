package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type item struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func newTestCache(t *testing.T) (*Cache, *miniredis.Miniredis) {
	t.Helper()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	return New(client), srv
}

func TestCache_Get(t *testing.T) {
	ctx := context.Background()

	t.Run("returns parsed data from cache", func(t *testing.T) {
		c, srv := newTestCache(t)
		_ = srv.Set("key", `{"id":1,"name":"Test"}`)

		var got item
		if !c.Get(ctx, "key", &got) {
			t.Fatal("expected cache hit")
		}
		if got != (item{ID: 1, Name: "Test"}) {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("reports a miss for absent keys", func(t *testing.T) {
		c, _ := newTestCache(t)

		var got item
		if c.Get(ctx, "missing", &got) {
			t.Error("expected cache miss")
		}
	})

	t.Run("reports a miss when Redis is unavailable", func(t *testing.T) {
		var got item
		if New(nil).Get(ctx, "key", &got) {
			t.Error("expected cache miss")
		}
	})

	t.Run("reports a miss on Redis error (graceful degradation)", func(t *testing.T) {
		c, srv := newTestCache(t)
		srv.Close()

		var got item
		if c.Get(ctx, "key", &got) {
			t.Error("expected cache miss")
		}
	})
}

func TestCache_Set(t *testing.T) {
	ctx := context.Background()

	t.Run("sets data with TTL", func(t *testing.T) {
		c, srv := newTestCache(t)

		c.Set(ctx, "key", item{ID: 1}, 5*time.Minute)

		if got, _ := srv.Get("key"); got != `{"id":1,"name":""}` {
			t.Errorf("got %q", got)
		}
		if ttl := srv.TTL("key"); ttl != 5*time.Minute {
			t.Errorf("got TTL %v", ttl)
		}
	})

	t.Run("sets data without TTL", func(t *testing.T) {
		c, srv := newTestCache(t)

		c.Set(ctx, "key", item{ID: 1}, 0)

		if !srv.Exists("key") || srv.TTL("key") != 0 {
			t.Error("expected key without expiration")
		}
	})

	t.Run("does not fail when Redis is unavailable", func(t *testing.T) {
		New(nil).Set(ctx, "key", item{}, time.Minute)
	})
}

func TestCache_Del(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes a key", func(t *testing.T) {
		c, srv := newTestCache(t)
		_ = srv.Set("key", "value")

		c.Del(ctx, "key")

		if srv.Exists("key") {
			t.Error("expected key to be deleted")
		}
	})

	t.Run("does not fail when Redis is unavailable", func(t *testing.T) {
		New(nil).Del(ctx, "key")
	})
}

func TestCache_InvalidatePattern(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes matching keys across multiple scan iterations", func(t *testing.T) {
		c, srv := newTestCache(t)
		for i := range 250 {
			_ = srv.Set(fmt.Sprintf("products:list:%d", i), "x")
		}
		_ = srv.Set("product:1", "x")

		c.InvalidatePattern(ctx, "products:*")

		if keys := srv.Keys(); len(keys) != 1 || keys[0] != "product:1" {
			t.Errorf("got remaining keys %v, want [product:1]", keys)
		}
	})

	t.Run("handles no matching keys", func(t *testing.T) {
		c, srv := newTestCache(t)
		_ = srv.Set("other", "x")

		c.InvalidatePattern(ctx, "products:*")

		if !srv.Exists("other") {
			t.Error("unrelated key must be kept")
		}
	})
}

func TestTTL(t *testing.T) {
	if got := TTL("product"); got != 5*time.Minute {
		t.Errorf("TTL(product) = %v", got)
	}
	if got := TTL("order"); got != time.Minute {
		t.Errorf("TTL(order) = %v", got)
	}
	if got := TTL("unknown"); got != 5*time.Minute {
		t.Errorf("TTL(unknown) = %v, want default", got)
	}
}
