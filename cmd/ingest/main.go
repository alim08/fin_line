package main

import (
    "context"
    "fmt"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/alim08/fin_line/pkg/config"
    "github.com/alim08/fin_line/pkg/logger"
    "github.com/alim08/fin_line/pkg/redisclient"
    "github.com/go-chi/chi/v5"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "go.uber.org/zap"
)

func main() {
    // 1. Load config
    cfg, err := config.Load()
    if err != nil {
        panic("config error: " + err.Error())
    }

    // 2. Init logger
    if err := logger.Init(); err != nil {
        panic("logger init: " + err.Error())
    }
    defer logger.Log.Sync()

    // 3. Connect to Redis
    rdb := redisclient.New(cfg.RedisURL)
    defer rdb.Close()

    // 4. Start Prometheus metrics endpoint
    go startMetricsServer(8082) // Use default metrics port

    // 5. Launch one ingestFeed per URL
    ctx, cancel := context.WithCancel(context.Background())
    for _, feed := range cfg.FeedURLs {
        go ingestFeed(ctx, rdb, feed)
    }

    // 6. Wait for shutdown signal
    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
    <-sigs
    logger.Log.Info("shutdown signal received, exiting")
    cancel()
    // give goroutines a moment to finish
    time.Sleep(500 * time.Millisecond)
}

func startMetricsServer(port int) {
    r := chi.NewRouter()
    r.Handle("/metrics", promhttp.Handler())
    addr := fmt.Sprintf(":%d", port)
    logger.Log.Info("metrics server listening", zap.String("addr", addr))
    http.ListenAndServe(addr, r) // errors are logged by default
}
