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
    // 1. Load configuration
    cfg, err := config.Load()
    if err != nil {
        panic("config load error: " + err.Error())
    }

    // 2. Initialize structured logging
    if err := logger.Init(); err != nil {
        panic("logger init error: " + err.Error())
    }
    defer logger.Log.Sync()

    // 3. Connect to Redis
    rdb := redisclient.New(cfg.RedisURL)
    defer rdb.Close()

    // 4. Launch cache-pub processor
    ctx, cancel := context.WithCancel(context.Background())
    go runCachePub(ctx, rdb)

    // 5. Graceful shutdown on SIGINT/SIGTERM
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
    <-stop

    logger.Log.Info("shutdown signal received, exiting")
    cancel()
    // allow in-flight messages to finish
    time.Sleep(200 * time.Millisecond)
}
