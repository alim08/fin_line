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
    // Load config & init logging
    cfg, err := config.Load()
    if err != nil {
        panic("config load: " + err.Error())
    }
    if err := logger.Init(); err != nil {
        panic("logger init: " + err.Error())
    }
    defer logger.Log.Sync()

    // Connect Redis
    rdb := redisclient.New(cfg.RedisURL)
    defer rdb.Close()

    // Cancellation & graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

    // Start normalization workers
    go startNormalization(ctx, rdb)

    // Block until signal
    <-sigs
    logger.Log.Info("shutdown signal received")
    cancel()
    // give a moment for in-flight work
    time.Sleep(200 * time.Millisecond)
}
