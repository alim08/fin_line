package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/alim08/fin_line/pkg/config"
    "github.com/alim08/fin_line/pkg/logger"
    "github.com/alim08/fin_line/pkg/redisclient"
)

func main() {
  // 1. Load configuration & init logging
  cfg, err := config.Load()
  if err != nil {
    panic("config load: " + err.Error())
  }
  if err := logger.Init(); err != nil {
    panic("logger init: " + err.Error())
  }
  defer logger.Log.Sync()

  // 2. Redis connection
  rdb := redisclient.New(cfg.RedisURL)
  defer rdb.Close()

  // 3. Run detector loop
  ctx, cancel := context.WithCancel(context.Background())
  go runAnomalyDetector(ctx, rdb, cfg)

  // 4. Wait for SIGINT/SIGTERM
  stop := make(chan os.Signal, 1)
  signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
  <-stop
  logger.Log.Info("shutting down anomaly detector")
  cancel()
  time.Sleep(200 * time.Millisecond)
}
