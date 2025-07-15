package main

import (
    "context"
    "encoding/json"
    "strconv"
    "time"

    "github.com/alim08/fin_line/pkg/logger"
    "github.com/alim08/fin_line/pkg/metrics"
    "github.com/alim08/fin_line/pkg/models"
    "github.com/alim08/fin_line/pkg/redisclient"
    "github.com/go-redis/redis/v8"
    "go.uber.org/zap"
)

// runCachePub subscribes to normalized events and publishes them to cache & channels.
func runCachePub(ctx context.Context, rdb *redisclient.Client) {
    logger.Log.Info("cachepub service started")

    // Read from the normalized:events stream
    lastID := "0-0"
    
    for {
        select {
        case <-ctx.Done():
            logger.Log.Info("runCachePub: context cancelled")
            return
        default:
            // Read from normalized:events stream
            res, err := rdb.Client().XRead(ctx, &redis.XReadArgs{
                Streams: []string{"normalized:events", lastID},
                Count:   100,
                Block:   500 * time.Millisecond,
            }).Result()
            
            if err != nil && err != redis.Nil {
                logger.Log.Warn("XREAD error", zap.Error(err))
                time.Sleep(200 * time.Millisecond)
                continue
            }
            
            if len(res) == 0 || len(res[0].Messages) == 0 {
                continue
            }
            
            for _, msg := range res[0].Messages {
                lastID = msg.ID
                
                // Parse the normalized tick
                var tick models.NormalizedTick
                if ticker, ok := msg.Values["ticker"].(string); ok {
                    tick.Ticker = ticker
                }
                if priceStr, ok := msg.Values["price"].(string); ok {
                    if price, err := strconv.ParseFloat(priceStr, 64); err == nil {
                        tick.Price = price
                    }
                }
                if tsMs, ok := msg.Values["ts_ms"].(string); ok {
                    if ts, err := strconv.ParseInt(tsMs, 10, 64); err == nil {
                        tick.Timestamp = ts
                    }
                }
                if sector, ok := msg.Values["sector"].(string); ok {
                    tick.Sector = sector
                }
                
                // Process the tick
                if err := publishTick(ctx, rdb, tick); err != nil {
                    logger.Log.Error("publishTick failed", zap.Error(err))
                    metrics.CachePubErrors.Inc()
                } else {
                    metrics.CachePubCounter.Inc()
                }
            }
        }
    }
}

// publishTick updates the latest-quote hash and publishes on quotes:pubsub.
func publishTick(ctx context.Context, rdb *redisclient.Client, tick models.NormalizedTick) error {
    // 1) Prepare Redis pipeline for atomicity & performance
    pipe := rdb.Client().Pipeline()

    // 2) Update hash: HSET quotes:latest:<ticker>
    hashKey := "quotes:latest:" + tick.Ticker
    pipe.HSet(ctx, hashKey, map[string]interface{}{
        "price": tick.Price,
        "ts_ms": tick.Timestamp,
    })

    // 3) Publish full JSON payload for subscribers
    payload, _ := json.Marshal(tick) // error unlikely; tick is well-typed
    pipe.Publish(ctx, "quotes:pubsub", payload)

    // 4) Execute pipeline with timeout
    execCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
    defer cancel()

    if _, err := pipe.Exec(execCtx); err != nil {
        return err
    }
    return nil
}
