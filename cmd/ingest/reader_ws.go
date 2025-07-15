package main

import (
    "context"
    "strings"

    "github.com/alim08/fin_line/pkg/logger"
    "github.com/alim08/fin_line/pkg/metrics"
    "github.com/cenkalti/backoff/v4"
    "github.com/gorilla/websocket"
    "go.uber.org/zap"
)

func ingestWebSocket(ctx context.Context, url string, events chan<- map[string]interface{}) {
    bo := backoff.WithContext(backoff.NewExponentialBackOff(), ctx)

    err := backoff.Retry(func() error {
        logger.Log.Info("dialing websocket", zap.String("url", url))
        conn, _, err := websocket.DefaultDialer.Dial(url, nil)
        if err != nil {
            logger.Log.Warn("ws dial error", zap.Error(err))
            return err
        }
        defer conn.Close()

        for {
            select {
            case <-ctx.Done():
                return backoff.Permanent(ctx.Err())
            default:
                var msg map[string]interface{}
                if err := conn.ReadJSON(&msg); err != nil {
                    logger.Log.Warn("ws read error", zap.Error(err))
                    return err
                }
                // drop if buffer full
                select {
                case events <- msg:
                default:
                    logger.Log.Warn("events chan full, dropping ws event")
                    metrics.IngestErrors.Inc()
                }
            }
        }
    }, bo)

    if err != nil && !strings.Contains(err.Error(), "context canceled") {
        logger.Log.Error("websocket reader stopped", zap.Error(err))
    }
}
