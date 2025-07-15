package main

import (
    "context"
    "time"

    "github.com/alim08/fin_line/pkg/logger"
    "github.com/alim08/fin_line/pkg/metrics"
    "github.com/alim08/fin_line/pkg/models"
    "github.com/alim08/fin_line/pkg/redisclient"
    "github.com/go-redis/redis/v8"
    "go.uber.org/zap"
)

// symbolMap could be loaded from config or DB; here's a stub.
var symbolMap = map[string]string{
    "BTCUSD": "BTCUSD",
    // add more mappings...
}

// sectorMap stub
var sectorMap = map[string]string{
    "BTCUSD": "crypto",
    // add more...
}

// Limits concurrent Normalize handlers
const maxWorkers = 50

func startNormalization(ctx context.Context, rdb *redisclient.Client) {
    logger.Log.Info("normalization worker started")
    sem := make(chan struct{}, maxWorkers)
    lastID := "0-0" // start reading from the very beginning

    for {
        // 1) Read up to 100 messages, wait up to 500ms
        res, err := rdb.Client().XRead(ctx, &redis.XReadArgs{
            Streams: []string{"raw:events", lastID},
            Count:   100,
            Block:   500 * time.Millisecond,
        }).Result()
        if err != nil && err != redis.Nil {
            logger.Log.Warn("XREAD error", zap.Error(err))
            time.Sleep(200 * time.Millisecond) // simple backoff
            continue
        }

        if len(res) == 0 || len(res[0].Messages) == 0 {
            continue
        }

        // 2) Process each message in parallel (bounded)
        for _, msg := range res[0].Messages {
            lastID = msg.ID // advance our cursor

            select {
            case sem <- struct{}{}:
                go func(m redis.XMessage) {
                    defer func() { <-sem }()
                    normalizeOne(ctx, rdb, m)
                }(msg)
            default:
                // Worker pool full: drop oldest to keep up
                logger.Log.Warn("normalize pool full, dropping message", zap.String("id", msg.ID))
                metrics.NormalizeErrors.Inc()
            }
        }
    }
}

func normalizeOne(ctx context.Context, rdb *redisclient.Client, msg redis.XMessage) {
    start := time.Now()
    defer metrics.NormalizeLatency.Observe(time.Since(start).Seconds())

    // 1) Convert raw map → typed RawTick
    raw, err := models.RawTickFromMap(msg.Values)
    if err != nil {
        logger.Log.Warn("raw parse error", zap.String("id", msg.ID), zap.Error(err))
        metrics.NormalizeErrors.Inc()
        return
    }

    // 2) Symbol mapping
    ticker, ok := symbolMap[raw.Symbol]
    if !ok {
        logger.Log.Warn("unknown symbol", zap.String("symbol", raw.Symbol))
        metrics.NormalizeErrors.Inc()
        return
    }

    // 3) Lookup sector (fallback to "unknown")
    sector := sectorMap[ticker]
    if sector == "" {
        sector = "unknown"
    }

    // 4) Build NormalizedTick
    norm := models.NormalizedTick{
        Ticker:    ticker,
        Price:     raw.Price,
        Timestamp: raw.Timestamp.UTC().UnixMilli(),
        Sector:    sector,
    }

    // 5) Write to normalized:events
    if err := rdb.AddToStream(ctx, "normalized:events", norm.ToMap()); err != nil {
        logger.Log.Error("failed to write normalized event", zap.Error(err))
        metrics.NormalizeErrors.Inc()
        return
    }
    metrics.NormalizeCounter.Inc()
}
