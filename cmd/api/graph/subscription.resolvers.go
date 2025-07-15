package graph

// This file will be automatically regenerated based on the schema, DO NOT EDIT.
// Add custom subscription resolvers in this file.

import (
	"context"
	"encoding/json"
	"time"
)

func (r *Resolver) QuoteUpdated(ctx context.Context, ticker *string) (<-chan *Quote, error) {
	// Create a channel for the subscription
	quoteChan := make(chan *Quote)

	// Subscribe to Redis channel for quote updates
	pubsub := r.redis.Client().Subscribe(ctx, "quotes")
	defer pubsub.Close()

	go func() {
		defer close(quoteChan)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := pubsub.ReceiveMessage(ctx)
				if err != nil {
					return
				}

				// Parse the quote data
				var quoteData map[string]interface{}
				if err := json.Unmarshal([]byte(msg.Payload), &quoteData); err != nil {
					continue
				}

				// Apply ticker filter if specified
				if ticker != nil {
					if quoteTicker, ok := quoteData["ticker"].(string); !ok || quoteTicker != *ticker {
						continue
					}
				}

				// Convert to model
				price, _ := quoteData["price"].(float64)
				timestamp, _ := quoteData["timestamp"].(float64)
				quoteTicker, _ := quoteData["ticker"].(string)
				sector, _ := quoteData["sector"].(string)

				quote := &Quote{
					Ticker:    quoteTicker,
					Price:     price,
					Timestamp: time.UnixMilli(int64(timestamp)),
					Sector:    &sector,
				}

				select {
				case quoteChan <- quote:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return quoteChan, nil
}

func (r *Resolver) AnomalyDetected(ctx context.Context, severity *string) (<-chan *Anomaly, error) {
	// Create a channel for the subscription
	anomalyChan := make(chan *Anomaly)

	// Subscribe to Redis channel for anomaly updates
	pubsub := r.redis.Client().Subscribe(ctx, "anomalies")
	defer pubsub.Close()

	go func() {
		defer close(anomalyChan)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := pubsub.ReceiveMessage(ctx)
				if err != nil {
					return
				}

				// Parse the anomaly data
				var anomalyData map[string]interface{}
				if err := json.Unmarshal([]byte(msg.Payload), &anomalyData); err != nil {
					continue
				}

				// Check if this is a deletion message
				if action, ok := anomalyData["action"].(string); ok && action == "delete" {
					continue // Skip deletion messages for now
				}

				// Apply severity filter if specified
				if severity != nil {
					if anomalySeverity, ok := anomalyData["severity"].(string); !ok || anomalySeverity != *severity {
						continue
					}
				}

				// Convert to model
				id, _ := anomalyData["id"].(string)
				anomalyTicker, _ := anomalyData["ticker"].(string)
				price, _ := anomalyData["price"].(float64)
				threshold, _ := anomalyData["threshold"].(float64)
				anomalyType, _ := anomalyData["type"].(string)
				timestamp, _ := anomalyData["timestamp"].(float64)
				anomalySeverity, _ := anomalyData["severity"].(string)

				anomaly := &Anomaly{
					ID:        id,
					Ticker:    anomalyTicker,
					Price:     price,
					Threshold: threshold,
					Type:      anomalyType,
					Timestamp: time.UnixMilli(int64(timestamp)),
					Severity:  anomalySeverity,
				}

				select {
				case anomalyChan <- anomaly:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return anomalyChan, nil
}

func (r *Resolver) MarketUpdate(ctx context.Context) (<-chan *MarketStats, error) {
	// Create a channel for the subscription
	statsChan := make(chan *MarketStats)

	// Subscribe to Redis channel for market updates
	pubsub := r.redis.Client().Subscribe(ctx, "market_updates")
	defer pubsub.Close()

	go func() {
		defer close(statsChan)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := pubsub.ReceiveMessage(ctx)
				if err != nil {
					return
				}

				// Parse the market stats data
				var statsData map[string]interface{}
				if err := json.Unmarshal([]byte(msg.Payload), &statsData); err != nil {
					continue
				}

				// Convert to model
				totalTickers, _ := statsData["total_tickers"].(float64)
				totalQuotes, _ := statsData["total_quotes"].(float64)
				avgPrice, _ := statsData["avg_price"].(float64)

				stats := &MarketStats{
					TotalTickers: int(totalTickers),
					TotalQuotes:  int(totalQuotes),
					AvgPrice:     &avgPrice,
					LastUpdate:   time.Now(),
				}

				select {
				case statsChan <- stats:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return statsChan, nil
} 