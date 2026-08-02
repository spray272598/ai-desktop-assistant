package redisx

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ai-desktop/assistant/internal/infrastructure/config"
)

// Client Redis 封装；Disabled 时所有操作 no-op 成功
type Client struct {
	rdb     *redis.Client
	enabled bool
}

func New(cfg config.RedisConfig) *Client {
	if !cfg.Enabled {
		return &Client{enabled: false}
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	if cfg.Host == "" {
		addr = "127.0.0.1:6379"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("[redis] ping failed (%v), disable redis\n", err)
		_ = rdb.Close()
		return &Client{enabled: false}
	}
	log.Printf("[redis] connected %s\n", addr)
	return &Client{rdb: rdb, enabled: true}
}

func (c *Client) Enabled() bool { return c != nil && c.enabled && c.rdb != nil }

func (c *Client) Close() error {
	if c != nil && c.rdb != nil {
		return c.rdb.Close()
	}
	return nil
}

func (c *Client) Set(ctx context.Context, key, val string, ttl time.Duration) error {
	if !c.Enabled() {
		return nil
	}
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	if !c.Enabled() {
		return "", redis.Nil
	}
	return c.rdb.Get(ctx, key).Result()
}

func (c *Client) Del(ctx context.Context, keys ...string) error {
	if !c.Enabled() {
		return nil
	}
	return c.rdb.Del(ctx, keys...).Err()
}

// AllowRate 简单固定窗口限流：key 在 window 内最多 limit 次
func (c *Client) AllowRate(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if limit <= 0 {
		return true, nil
	}
	if !c.Enabled() {
		return memoryRateAllow(key, limit, window), nil
	}
	pipe := c.rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return true, err // 失败放行，避免拖垮服务
	}
	n, _ := incr.Result()
	return n <= int64(limit), nil
}

// ---- memory fallback rate limit ----

type memBucket struct {
	count int
	exp   time.Time
}

var (
	memMu   sync.Mutex
	memRate = map[string]*memBucket{}
)

func memoryRateAllow(key string, limit int, window time.Duration) bool {
	memMu.Lock()
	defer memMu.Unlock()
	now := time.Now()
	b, ok := memRate[key]
	if !ok || now.After(b.exp) {
		memRate[key] = &memBucket{count: 1, exp: now.Add(window)}
		return true
	}
	b.count++
	return b.count <= limit
}
