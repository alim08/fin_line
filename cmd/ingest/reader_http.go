package main

import (
    "context"
    "encoding/json"
    "net/http"
    "time"

    "github.com/alim08/fin_line/pkg/logger"
    "github.com/alim08/fin_line/pkg/metrics"
    "go.uber.org/zap"
)

func ingestHTTP(ctx context.Context, url string, events chan<- map[string]interface{}) {
    client := &http.Client{
        Timeout: 5 * time.Second,
        Transport: &http.Transport{
            MaxIdleConns:        10,
            MaxIdleConnsPerHost: 5,
            IdleConnTimeout:     30 * time.Second,
        },
    }
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            resp, err := client.Get(url)
            if err != nil {
                logger.Log.Warn("http get failed", zap.String("url", url), zap.Error(err))
                metrics.IngestErrors.Inc()
                continue
            }
            if resp.StatusCode != http.StatusOK {
                logger.Log.Warn("non-200 from HTTP", zap.Int("code", resp.StatusCode))
                resp.Body.Close()
                metrics.IngestErrors.Inc()
                continue
            }

            var batch []map[string]interface{}
            dec := json.NewDecoder(resp.Body)
            if err := dec.Decode(&batch); err != nil {
                logger.Log.Warn("json decode error", zap.Error(err))
                resp.Body.Close()
                metrics.IngestErrors.Inc()
                continue
            }
            resp.Body.Close()

            for _, evt := range batch {
                select {
                case events <- evt:
                default:
                    logger.Log.Warn("events chan full, dropping http event")
                    metrics.IngestErrors.Inc()
                }
            }
        }
    }
}
