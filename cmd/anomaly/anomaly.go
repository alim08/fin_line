package main

import (
  "context"
  "encoding/json"
  "math"
  "sync"

  "github.com/alim08/fin_line/pkg/config"
  "github.com/alim08/fin_line/pkg/logger"
  "github.com/alim08/fin_line/pkg/metrics"
  "github.com/alim08/fin_line/pkg/models"
  "github.com/alim08/fin_line/pkg/redisclient"
  "github.com/go-redis/redis/v8"
  "go.uber.org/zap"
)

// rollingWindow holds a fixed-size ring buffer for O(1) mean/stddev.
type rollingWindow struct {
  buf        []float64
  sum, sqsum float64
  idx        int
  full       bool
}

func newWindow(size int) *rollingWindow {
  return &rollingWindow{buf: make([]float64, size)}
}

func (w *rollingWindow) add(x float64) {
  if w.full {
    old := w.buf[w.idx]
    w.sum -= old
    w.sqsum -= old * old
  }
  w.buf[w.idx] = x
  w.sum += x
  w.sqsum += x * x
  w.idx = (w.idx + 1) % len(w.buf)
  if w.idx == 0 {
    w.full = true
  }
}

func (w *rollingWindow) stats() (mean, std float64) {
  n := float64(len(w.buf))
  if !w.full {
    n = float64(w.idx)
  }
  if n == 0 {
    return 0, 0
  }
  mean = w.sum / n
  variance := (w.sqsum / n) - (mean * mean)
  if variance < 0 {
    variance = 0
  }
  std = math.Sqrt(variance)
  return
}

func runAnomalyDetector(ctx context.Context, rdb *redisclient.Client, cfg *config.Config) {
  logger.Log.Info("anomaly detector started")
  pubsub := rdb.Client().Subscribe(ctx, "quotes:pubsub")
  defer pubsub.Close()

  // One window per ticker, synchronized
  windows := make(map[string]*rollingWindow)
  mu := sync.Mutex{}

  for {
    select {
    case <-ctx.Done():
      logger.Log.Info("anomaly detector stopping")
      return

    case msg, ok := <-pubsub.Channel():
      if !ok {
        logger.Log.Warn("quotes:pubsub closed")
        return
      }

      var tick models.NormalizedTick
      if err := json.Unmarshal([]byte(msg.Payload), &tick); err != nil {
        logger.Log.Warn("invalid tick JSON", zap.Error(err))
        metrics.AnomalyErrors.Inc()
        continue
      }

      // Ensure window exists
      mu.Lock()
      w, exists := windows[tick.Ticker]
      if !exists {
        w = newWindow(cfg.AnomalyWindowSize)
        windows[tick.Ticker] = w
      }
      mu.Unlock()

      // Update window & compute z-score
      w.add(tick.Price)
      mean, std := w.stats()
      if std == 0 {
        continue // no variation yet
      }
      z := math.Abs((tick.Price - mean) / std)
      if z >= cfg.AnomalyThreshold {
        // Build event
        event := models.Anomaly{
          Ticker:    tick.Ticker,
          Price:     tick.Price,
          ZScore:    z,
          Timestamp: tick.Timestamp,
        }
        emitAnomaly(ctx, rdb, event)
      }
    }
  }
}

func emitAnomaly(ctx context.Context, rdb *redisclient.Client, a models.Anomaly) {
  // 1) Stream entry
  val := map[string]interface{}{
    "ticker": a.Ticker,
    "price":  a.Price,
    "z":      a.ZScore,
    "ts_ms":  a.Timestamp,
  }
  if err := rdb.AddToStream(ctx, "anomalies:stream", val); err != nil {
    logger.Log.Error("XADD anomalies:stream failed", zap.Error(err))
    metrics.AnomalyErrors.Inc()
  }

  // 2) Sorted set (for range queries)
  score := float64(a.Timestamp)
  if err := rdb.Client().ZAdd(ctx,
    "anomalies:"+a.Ticker,
    &redis.Z{Score: score, Member: toJSON(val)},
  ).Err(); err != nil {
    logger.Log.Error("ZADD anomalies set failed", zap.Error(err))
    metrics.AnomalyErrors.Inc()
  } else {
    metrics.AnomalyCounter.Inc()
  }
}

func toJSON(v interface{}) string {
  b, _ := json.Marshal(v)
  return string(b)
}
