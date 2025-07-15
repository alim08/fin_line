package main

import (
    "context"
    "strings"

    "github.com/alim08/fin_line/pkg/logger"
    "github.com/alim08/fin_line/pkg/metrics"
    "github.com/alim08/fin_line/pkg/redisclient"
    "go.uber.org/zap"
)

func ingestFeed(ctx context.Context, rdb *redisclient.Client, feedURL string) {
    logger.Log.Info("starting ingestFeed", zap.String("url", feedURL))

    // 1. Buffer up to 1k events before blocking the reader
    events := make(chan map[string]interface{}, 1000)

    // 2. Start 5 writers to Redis
    for i := 0; i < 5; i++ {
        go func(id int) {
            for {
                select {
                case <-ctx.Done():
                    logger.Log.Info("writer exiting", zap.Int("worker", id))
                    return
                case evt, ok := <-events:
                    if !ok {
                        return
                    }
                    if err := rdb.AddToStream(ctx, "raw:events", evt); err != nil {
                        logger.Log.Warn("stream write failed", zap.Error(err))
                        metrics.IngestErrors.Inc()
                        continue
                    }
                    metrics.IngestCounter.Inc()
                }
            }
        }(i)
    }

    // 3. Dispatch to the appropriate reader
    if strings.HasPrefix(feedURL, "ws://") || strings.HasPrefix(feedURL, "wss://") {
        ingestWebSocket(ctx, feedURL, events)
    } else {
        ingestHTTP(ctx, feedURL, events)
    }

    // 4. Clean up
    close(events)
    logger.Log.Info("ingestFeed terminated", zap.String("url", feedURL))
}
