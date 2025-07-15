package redisclient

import (
  "context"
  "time"
  "sync/atomic"
  "errors"

  "github.com/go-redis/redis/v8"
  "github.com/cenkalti/backoff/v4"
  "github.com/alim08/fin_line/pkg/metrics"
  "github.com/alim08/fin_line/pkg/logger"
  "go.uber.org/zap"
)

var (
  ErrCircuitBreakerOpen = errors.New("circuit breaker is open")
  ErrTimeout = errors.New("operation timeout")
)

type Client struct {
  rdb *redis.Client
  // Circuit breaker state
  failureCount int64
  lastFailure  int64
  state        int32 // 0: closed, 1: open, 2: half-open
}

// New constructs a Client with sensible defaults & retry logic
func New(redisURL string) *Client {
  opt, err := redis.ParseURL(redisURL)
  if err != nil {
    panic("invalid REDIS_URL: " + err.Error())
  }
  // Tune PoolSize to number of CPU cores × factor
  opt.PoolSize = 20
  opt.MinIdleConns = 5
  opt.MaxRetries = 3
  opt.DialTimeout = 5 * time.Second
  opt.ReadTimeout = 3 * time.Second
  opt.WriteTimeout = 3 * time.Second
  opt.IdleTimeout = 5 * time.Minute
  rdb := redis.NewClient(opt)
  return &Client{rdb: rdb}
}

// withMetrics wraps operations with metrics collection
func (c *Client) withMetrics(operation string, fn func() error) error {
  start := time.Now()
  err := fn()
  duration := time.Since(start).Seconds()
  
  metrics.RedisOperationDuration.WithLabelValues(operation, getStatus(err)).Observe(duration)
  if err != nil {
    metrics.RedisErrors.WithLabelValues(operation).Inc()
  }
  
  return err
}

// getStatus returns "success" or "error" for metrics
func getStatus(err error) string {
  if err != nil {
    return "error"
  }
  return "success"
}

// checkCircuitBreaker checks if circuit breaker should be opened/closed
func (c *Client) checkCircuitBreaker(err error) {
  if err != nil {
    atomic.AddInt64(&c.failureCount, 1)
    atomic.StoreInt64(&c.lastFailure, time.Now().Unix())
    
    // Open circuit breaker after 5 consecutive failures
    if atomic.LoadInt64(&c.failureCount) >= 5 {
      atomic.CompareAndSwapInt32(&c.state, 0, 1) // closed -> open
      logger.Log.Warn("circuit breaker opened", zap.String("operation", "redis"))
    }
  } else {
    // Reset failure count on success
    atomic.StoreInt64(&c.failureCount, 0)
    atomic.CompareAndSwapInt32(&c.state, 1, 2) // open -> half-open
  }
}

// AddToStream appends into a Redis Stream with retry/backoff
func (c *Client) AddToStream(ctx context.Context, stream string, values map[string]interface{}) error {
  return c.withMetrics("xadd", func() error {
    // Check circuit breaker
    if atomic.LoadInt32(&c.state) == 1 {
      return ErrCircuitBreakerOpen
    }
    
    op := func() error {
      // 100ms timeout per attempt
      ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
      defer cancel()
      _, err := c.rdb.XAdd(ctx, &redis.XAddArgs{
        Stream: stream,
        Values: values,
      }).Result()
      
      c.checkCircuitBreaker(err)
      return err
    }
    // exponential backoff: max 3 retries
    return backoff.Retry(op, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3))
  })
}

// XRead reads from Redis streams with timeout
func (c *Client) XRead(ctx context.Context, args *redis.XReadArgs) *redis.XStreamSliceCmd {
  return c.rdb.XRead(ctx, args)
}

// Publish wraps rdb.Publish with a short timeout
func (c *Client) Publish(ctx context.Context, channel string, msg interface{}) error {
  return c.withMetrics("publish", func() error {
    if atomic.LoadInt32(&c.state) == 1 {
      return ErrCircuitBreakerOpen
    }
    
    ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
    defer cancel()
    err := c.rdb.Publish(ctx, channel, msg).Err()
    c.checkCircuitBreaker(err)
    return err
  })
}

// HSet sets a hash with retry
func (c *Client) HSet(ctx context.Context, key string, values map[string]interface{}) error {
  return c.withMetrics("hset", func() error {
    if atomic.LoadInt32(&c.state) == 1 {
      return ErrCircuitBreakerOpen
    }
    
    // same pattern as AddToStream
    op := func() error {
      ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
      defer cancel()
      err := c.rdb.HSet(ctx, key, values).Err()
      c.checkCircuitBreaker(err)
      return err
    }
    return backoff.Retry(op, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3))
  })
}

// HGetAll retrieves all fields from a hash
func (c *Client) HGetAll(ctx context.Context, key string) *redis.StringStringMapCmd {
  return c.rdb.HGetAll(ctx, key)
}

// Subscribe creates a pub/sub subscription
func (c *Client) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
  return c.rdb.Subscribe(ctx, channels...)
}

// Close closes the underlying connection pool
func (c *Client) Close() error {
  return c.rdb.Close()
}

// Client returns the underlying Redis client for direct access
func (c *Client) Client() *redis.Client {
  return c.rdb
}


