package main

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/alim08/fin_line/pkg/config"
	"github.com/alim08/fin_line/pkg/logger"
	"github.com/alim08/fin_line/pkg/metrics"
	"github.com/alim08/fin_line/pkg/redisclient"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		panic("config error: " + err.Error())
	}

	// Initialize logger
	if err := logger.Init(); err != nil {
		panic("logger init: " + err.Error())
	}
	defer logger.Log.Sync()

	// Connect to Redis
	rdb := redisclient.New(cfg.RedisURL)
	defer rdb.Close()

	// Start metrics server
	go startMetricsServer()

	// Start archival process
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run archival every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	logger.Log.Info("archival service started")

	for {
		select {
		case <-ctx.Done():
			logger.Log.Info("archival service shutting down")
			return
		case <-ticker.C:
			if err := runArchival(ctx, rdb); err != nil {
				logger.Log.Error("archival failed", zap.Error(err))
				metrics.ArchivalErrorCounter.Inc()
			} else {
				logger.Log.Info("archival completed successfully")
				metrics.ArchivalSuccessCounter.Inc()
			}
		}
	}
}

func runArchival(ctx context.Context, rdb *redisclient.Client) error {
	// Archive old quotes (older than 7 days)
	if err := archiveOldQuotes(ctx, rdb); err != nil {
		return err
	}

	// Archive old anomalies (older than 30 days)
	if err := archiveOldAnomalies(ctx, rdb); err != nil {
		return err
	}

	// Archive old raw events (older than 1 day)
	if err := archiveOldRawEvents(ctx, rdb); err != nil {
		return err
	}

	return nil
}

func archiveOldQuotes(ctx context.Context, rdb *redisclient.Client) error {
	// Archive quotes older than 7 days
	cutoff := time.Now().AddDate(0, 0, -7).UnixMilli()
	
	// Get old quotes from normalized:quotes stream
	args := &redis.XReadArgs{
		Streams: []string{"normalized:quotes", "0"},
		Count:   1000,
		Block:   100 * time.Millisecond,
	}

	streams, err := rdb.Client().XRead(ctx, args).Result()
	if err != nil && err != redis.Nil {
		return err
	}

	if len(streams) > 0 && len(streams[0].Messages) > 0 {
		for _, msg := range streams[0].Messages {
			// Parse timestamp from message ID
			tsMs, _ := msg.Values["ts_ms"].(string)
			if tsMs == "" {
				continue
			}

			timestamp, err := strconv.ParseInt(tsMs, 10, 64)
			if err != nil {
				continue
			}

			// If message is old enough, archive it
			if timestamp < cutoff {
				// Archive to long-term storage (e.g., database, file system)
				if err := archiveQuote(msg); err != nil {
					logger.Log.Error("failed to archive quote", zap.Error(err), zap.String("id", msg.ID))
				} else {
					// Remove from Redis stream
					rdb.Client().XDel(ctx, "normalized:quotes", msg.ID)
				}
			}
		}
	}

	return nil
}

func archiveOldAnomalies(ctx context.Context, rdb *redisclient.Client) error {
	// Archive anomalies older than 30 days
	cutoff := time.Now().AddDate(0, 0, -30).UnixMilli()

	// Get old anomalies from anomalies list
	anomalies, err := rdb.Client().LRange(ctx, "anomalies", 0, -1).Result()
	if err != nil && err != redis.Nil {
		return err
	}

	for _, anomalyStr := range anomalies {
		var anomalyData map[string]interface{}
		if err := json.Unmarshal([]byte(anomalyStr), &anomalyData); err != nil {
			continue
		}

		// Get timestamp
		var timestamp int64
		if tsFloat, ok := anomalyData["timestamp"].(float64); ok {
			timestamp = int64(tsFloat)
		} else if tsStr, ok := anomalyData["timestamp"].(string); ok {
			if parsedTs, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
				timestamp = parsedTs
			} else {
				continue
			}
		} else {
			continue
		}

		// If anomaly is old enough, archive it
		if timestamp < cutoff {
			// Archive to long-term storage
			if err := archiveAnomaly(anomalyData); err != nil {
				logger.Log.Error("failed to archive anomaly", zap.Error(err))
			} else {
				// Remove from Redis list
				rdb.Client().LRem(ctx, "anomalies", 1, anomalyStr)
			}
		}
	}

	return nil
}

func archiveOldRawEvents(ctx context.Context, rdb *redisclient.Client) error {
	// Archive raw events older than 1 day
	cutoff := time.Now().AddDate(0, 0, -1).UnixMilli()

	// Get old raw events from raw:events stream
	args := &redis.XReadArgs{
		Streams: []string{"raw:events", "0"},
		Count:   1000,
		Block:   100 * time.Millisecond,
	}

	streams, err := rdb.Client().XRead(ctx, args).Result()
	if err != nil && err != redis.Nil {
		return err
	}

	if len(streams) > 0 && len(streams[0].Messages) > 0 {
		for _, msg := range streams[0].Messages {
			// Parse timestamp from message ID
			tsMs, _ := msg.Values["timestamp"].(string)
			if tsMs == "" {
				continue
			}

			timestamp, err := strconv.ParseInt(tsMs, 10, 64)
			if err != nil {
				continue
			}

			// If message is old enough, archive it
			if timestamp < cutoff {
				// Archive to long-term storage
				if err := archiveRawEvent(msg); err != nil {
					logger.Log.Error("failed to archive raw event", zap.Error(err), zap.String("id", msg.ID))
				} else {
					// Remove from Redis stream
					rdb.Client().XDel(ctx, "raw:events", msg.ID)
				}
			}
		}
	}

	return nil
}

// Placeholder functions for actual archival implementation
func archiveQuote(msg redis.XMessage) error {
	// TODO: Implement actual archival to database or file system
	logger.Log.Info("archiving quote", zap.String("id", msg.ID))
	return nil
}

func archiveAnomaly(data map[string]interface{}) error {
	// TODO: Implement actual archival to database or file system
	logger.Log.Info("archiving anomaly", zap.String("id", data["id"].(string)))
	return nil
}

func archiveRawEvent(msg redis.XMessage) error {
	// TODO: Implement actual archival to database or file system
	logger.Log.Info("archiving raw event", zap.String("id", msg.ID))
	return nil
}

func startMetricsServer() {
	// TODO: Implement metrics server
	logger.Log.Info("metrics server started")
} 